package hugo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/state"
	"git.home.luguber.info/inful/docbuilder/internal/version"
	"git.home.luguber.info/inful/docbuilder/internal/versioning"
)

const skipReasonNoChanges = "no_changes"

// Generator handles Hugo site generation with the Docsy theme.
type Generator struct {
	config    *config.Config
	outputDir string // final output dir
	stageDir  string // ephemeral staging dir for current build
	// optional instrumentation callbacks (not exported)
	onPageRendered func()
	recorder       metrics.Recorder
	observer       models.BuildObserver // high-level observer (decouples metrics recorder)
	renderer       models.Renderer      // pluggable renderer abstraction (defaults to BinaryRenderer)
	// editLinkResolver centralizes per-page edit link resolution
	editLinkResolver *EditLinkResolver
	// stateManager (optional) allows stages to persist per-repo metadata (doc counts, hashes, commits) without daemon-specific code.
	stateManager state.RepositoryMetadataWriter
	// keepStaging preserves staging directory on failure for debugging (set via WithKeepStaging)
	keepStaging bool
	// onDocumentReady (optional) is invoked once per non-generated document
	// immediately after it has been written to disk. It is the daemon-mode
	// hook for pushing document content to a downstream consumer (e.g.
	// ragabast for async ingest). nil disables the hook entirely. The
	// callback receives the raw markdown bytes and the Hugo content path.
	onDocumentReady func(content []byte, path string)
}

// NewGenerator creates a new Hugo site generator.
func NewGenerator(cfg *config.Config, outputDir string) *Generator {
	g := &Generator{config: cfg, outputDir: filepath.Clean(outputDir), recorder: metrics.NoopRecorder{}}
	// Renderer defaults to nil; models.StageRunHugo will use BinaryRenderer when needed.
	// Use WithRenderer to inject custom/test renderers.
	// Default observer bridges to recorder until dedicated observers added.
	g.observer = models.RecorderObserver{Recorder: g.recorder}
	// Initialize resolver eagerly (cheap) to simplify call sites.
	g.editLinkResolver = NewEditLinkResolver(cfg)

	// Log Hugo configuration
	slog.Debug("Hugo generator created",
		"output_dir", outputDir)

	return g
}

// EditLinkResolver exposes the internal resolver for transforms (read-only behavior).
func (g *Generator) EditLinkResolver() interface{ Resolve(docs.DocFile) string } {
	return g.editLinkResolver
}

// WithStateManager injects an optional state manager for persistence of discovery/build metadata.
// Accepts any type implementing state.RepositoryMetadataWriter (e.g., state.ServiceAdapter).
func (g *Generator) WithStateManager(sm state.RepositoryMetadataWriter) *Generator {
	g.stateManager = sm
	return g
}

// persistRepoCommits records the commit each repository was built at, so the
// next build can tell whether a repository changed. The state manager is
// optional; without it no commit is recorded and the next build always
// rebuilds.
func (g *Generator) persistRepoCommits(bs *models.BuildState) {
	if g.stateManager == nil {
		return
	}
	tracker, ok := any(g.stateManager).(interface {
		SetRepoLastCommit(url, name, branch, commit string)
	})
	if !ok {
		return
	}
	for i := range bs.Git.Repositories {
		repo := &bs.Git.Repositories[i]
		commit, ok := bs.Git.PostHeads[repo.Name]
		if !ok || commit == "" {
			continue
		}
		tracker.SetRepoLastCommit(repo.URL, repo.Name, repo.Branch, commit)
	}
}

// WithKeepStaging enables preservation of staging directory on failure for debugging.
// When enabled, staging directory will not be cleaned up if Hugo build fails.
func (g *Generator) WithKeepStaging(keep bool) *Generator {
	g.keepStaging = keep
	if keep {
		slog.Debug("Staging directory preservation enabled for debugging")
	}
	return g
}

// existingSiteValidForSkip performs a lightweight integrity probe of the current output
// directory to decide whether an early in-run skip (after clone stage) is safe.
// We only allow the skip when:
//   - build-report.json exists (file, not directory)
//   - public/ directory exists and is non-empty
//   - content/ directory has at least one markdown file
//
// Failing any check returns false, forcing the pipeline to continue so content is regenerated.
func (g *Generator) existingSiteValidForSkip() bool {
	prev, ok := g.readPreviousBuildReport()
	if !ok {
		return false
	}
	if !g.previousReportAllowsSkip(prev) {
		return false
	}
	// Align with skip evaluation: if a daemon state manager is present and its stored
	// config hash disagrees with the current config snapshot, do not early-skip.
	// Otherwise the daemon can get stuck serving a valid site while its state never
	// converges, causing subsequent skip evaluations to keep failing.
	if g.stateManager != nil {
		type configHashReader interface{ GetLastConfigHash() string }
		if r, ok := any(g.stateManager).(configHashReader); ok {
			currentHash := g.ComputeConfigHashForPersistence()
			storedHash := r.GetLastConfigHash()
			if currentHash == "" || storedHash == "" || currentHash != storedHash {
				return false
			}
		}
	}
	if !g.outputHasPublicIndex() {
		return false
	}
	return g.outputHasNonRootMarkdownContent()
}

func (g *Generator) readPreviousBuildReport() (*models.BuildReport, bool) {
	reportPath := filepath.Join(g.outputDir, "build-report.json")
	if fi, err := os.Stat(reportPath); err != nil || fi.IsDir() {
		return nil, false
	}
	// Parse the previous build report to validate it's compatible with the current
	// binary/config. If we cannot parse the report, treat the output as unsafe to
	// skip (we'd rather rebuild than serve an empty/partial site).
	var prev models.BuildReport
	// #nosec G304 -- reportPath is derived from the configured output directory.
	b, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, false
	}
	if err := json.Unmarshal(b, &prev); err != nil {
		return nil, false
	}
	return &prev, true
}

func (g *Generator) previousReportAllowsSkip(prev *models.BuildReport) bool {
	// Only consider skipping if the previous build wasn't a failure.
	if prev.Outcome != models.OutcomeSuccess && prev.Outcome != models.OutcomeWarning {
		return false
	}
	// Do not early-skip when the previous build discovered zero documentation files.
	// An empty prior build is frequently a sign of misconfiguration or a transient discovery
	// issue; skipping would cause the daemon to keep serving an empty site forever.
	if prev.Files <= 0 {
		return false
	}
	// If the prior report recorded a config hash, it must match current.
	// Missing hash is treated as unsafe (forces rebuild after older versions).
	currentHash := g.ComputeConfigHash()
	if currentHash == "" || prev.ConfigHash == "" || prev.ConfigHash != currentHash {
		return false
	}
	// If the prior report recorded tool versions, ensure they still match.
	// Missing versions are treated as unsafe to avoid skipping across upgrades.
	if prev.DocBuilderVersion == "" || prev.DocBuilderVersion != version.Version {
		return false
	}
	if prev.HugoVersion != "" {
		if cur := models.DetectHugoVersion(context.Background()); cur != "" && cur != prev.HugoVersion {
			return false
		}
	}
	return true
}

func (g *Generator) outputHasPublicIndex() bool {
	publicDir := filepath.Join(g.outputDir, "public")
	if fi, err := os.Stat(publicDir); err != nil || !fi.IsDir() {
		return false
	}
	// Require a real rendered entrypoint; public/ containing only directories or
	// temporary artifacts can cause 404s for "/".
	if fi, err := os.Stat(filepath.Join(publicDir, "index.html")); err != nil || fi.IsDir() {
		return false
	}
	entries, err := os.ReadDir(publicDir)
	if err != nil || len(entries) == 0 {
		return false
	}
	return true
}

func (g *Generator) outputHasNonRootMarkdownContent() bool {
	contentDir := filepath.Join(g.outputDir, "content")
	if fi, err := os.Stat(contentDir); err != nil || !fi.IsDir() {
		return false
	}
	foundAny := false
	foundNonRoot := false
	if err := filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if foundAny && foundNonRoot {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		foundAny = true
		// Reject an output that only contains the global scaffold content/_index.md.
		// A real docs build should have at least one repo/section/page markdown file.
		rel := strings.TrimPrefix(path, contentDir+string(os.PathSeparator))
		rel = filepath.ToSlash(rel)
		if rel != "_index.md" {
			foundNonRoot = true
		}
		return nil
	}); err != nil {
		return false
	}
	return foundAny && foundNonRoot
}

// Config exposes the underlying configuration (read-only usage by themes).

// SetRecorder injects a metrics recorder (optional). Returns the generator for chaining.
func (g *Generator) SetRecorder(r metrics.Recorder) *Generator {
	if r == nil {
		g.recorder = metrics.NoopRecorder{}
		g.observer = models.RecorderObserver{Recorder: g.recorder}
		return g
	}
	g.recorder = r
	g.observer = models.RecorderObserver{Recorder: r}
	return g
}

// buildPipeline returns the stages of a build.
//
// When cloneAndDiscover is false the repositories are not cloned and
// the documents are not discovered: the build starts from documents
// that are already known. The stages after that are the same either
// way, so they are written once.
func buildPipeline(cloneAndDiscover bool) []models.StageDef {
	p := models.NewPipeline().
		Add(models.StagePrepareOutput, stages.StagePrepareOutput)
	if cloneAndDiscover {
		p = p.
			Add(models.StageCloneRepos, stages.StageCloneRepos).
			Add(models.StageDiscoverDocs, stages.StageDiscoverDocs)
	}
	return p.
		Add(models.StageGenerateConfig, stages.StageGenerateConfig).
		Add(models.StageCopyContent, stages.StageCopyContent).
		Add(models.StageRunHugo, stages.StageRunHugo).
		Add(models.StagePostProcess, stages.StagePostProcess).
		Build()
}

// GenerateSite creates a complete Hugo site from discovered documentation.
func (g *Generator) GenerateSite(docFiles []docs.DocFile) error {
	_, err := g.GenerateSiteWithReport(docFiles)
	return err
}

// GenerateSiteWithReport performs site generation (background context) and returns a BuildReport with metrics.
// Prefer GenerateSiteWithReportContext when you have a caller context supporting cancellation/timeouts.
func (g *Generator) GenerateSiteWithReport(docFiles []docs.DocFile) (*models.BuildReport, error) {
	return g.GenerateSiteWithReportContext(context.Background(), docFiles)
}

// GenerateSiteWithReportContext performs site generation honoring the provided context for cancellation.
func (g *Generator) GenerateSiteWithReportContext(ctx context.Context, docFiles []docs.DocFile) (*models.BuildReport, error) {
	slog.Info("Starting Hugo site generation", slog.String("output", g.outputDir), slog.Int("files", len(docFiles)))
	if err := g.beginStaging(); err != nil {
		return nil, err
	}
	repoSet := map[string]struct{}{}
	for i := range docFiles {
		f := &docFiles[i]
		repoSet[f.Repository] = struct{}{}
	}
	report := models.NewBuildReport(ctx, len(repoSet), len(docFiles))
	// Populate observability enrichment fields
	report.PipelineVersion = 1
	report.EffectiveRenderMode = string(config.ResolveEffectiveRenderMode(g.config))
	// Direct generation path bypasses clone stage entirely.
	report.CloneStageSkipped = true
	// instrumentation hook to count rendered pages
	g.onPageRendered = func() { report.RenderedPages++ }

	bs := models.NewBuildState(g, docFiles, report)

	pipeline := buildPipeline(false)

	if err := stages.RunStages(ctx, bs, pipeline); err != nil {
		// cleanup staging dir on failure
		g.abortStaging()
		// If clone stage executed (presence of durations entry) flip flag.
		if _, ok := report.StageDurations[string(models.StageCloneRepos)]; ok {
			report.CloneStageSkipped = false
		}
		return nil, err
	}

	// Compute doc files hash (direct generation path bypasses discovery stage where this normally occurs)
	if report.DocFilesHash == "" && len(docFiles) > 0 {
		// Compute isSingleRepo from docFiles
		repoSet := make(map[string]struct{})
		for i := range docFiles {
			repoSet[docFiles[i].Repository] = struct{}{}
		}
		isSingleRepo := len(repoSet) == 1

		paths := make([]string, 0, len(docFiles))
		for i := range docFiles {
			f := &docFiles[i]
			paths = append(paths, f.GetHugoPath(isSingleRepo))
		}
		sort.Strings(paths)
		h := sha256.New()
		for _, p := range paths {
			_, _ = h.Write([]byte(p))
			_, _ = h.Write([]byte{0})
		}
		report.DocFilesHash = hex.EncodeToString(h.Sum(nil))
	}

	// Stage durations already written directly to report.

	report.DeriveOutcome()
	report.Finish()
	if err := g.finalizeStaging(); err != nil {
		return nil, fmt.Errorf("finalize staging: %w", err)
	}

	// Verify public directory exists and log details
	publicDir := filepath.Join(g.outputDir, "public")
	if stat, err := os.Stat(publicDir); err == nil && stat.IsDir() {
		// Count files in public directory
		var fileCount int
		_ = filepath.Walk(publicDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				fileCount++
			}
			return nil
		})
		slog.Info("Public directory verified after finalization",
			slog.String("path", publicDir),
			slog.Int("files", fileCount),
			slog.Time("modified", stat.ModTime()),
			slog.Bool("static_rendered", report.StaticRendered))
	} else {
		slog.Warn("Public directory not found after finalization",
			slog.String("expected_path", publicDir),
			slog.String("output_dir", g.outputDir),
			slog.Bool("static_rendered", report.StaticRendered),
			slog.String("error", fmt.Sprintf("%v", err)))
	}

	if _, ok := report.StageDurations[string(models.StageCloneRepos)]; ok {
		report.CloneStageSkipped = false
	}
	// Persist report (best effort) inside final output directory
	if err := report.Persist(g.outputDir); err != nil {
		slog.Warn("Failed to persist build report", "error", err)
	}
	// record build-level metrics
	if g.recorder != nil {
		g.recorder.ObserveBuildDuration(report.End.Sub(report.Start))
		g.recorder.IncBuildOutcome(metrics.BuildOutcomeLabel(report.Outcome))
	}
	slog.Info("Hugo site generation completed",
		slog.String("output", g.outputDir),
		slog.Int("repos", report.Repositories),
		slog.Int("files", report.Files),
		slog.Int("errors", len(report.Errors)),
		slog.String("outcome", string(report.Outcome)))
	return report, nil
}

// (Helper methods split into separate files for maintainability.)

// GenerateFullSite clones repositories, discovers documentation, then executes the standard generation stages.
// repositories: list of repositories to process. workspaceDir: directory for git operations (created if missing).
func (g *Generator) GenerateFullSite(ctx context.Context, repositories []config.Repository, workspaceDir string) (*models.BuildReport, error) {
	report := models.NewBuildReport(ctx, 0, 0) // counts filled after discovery
	report.PipelineVersion = 1
	report.EffectiveRenderMode = string(config.ResolveEffectiveRenderMode(g.config))
	// By default full site path includes clone stage; mark skipped=false (may stay false)
	report.CloneStageSkipped = false
	if err := g.beginStaging(); err != nil {
		return nil, err
	}
	g.onPageRendered = func() { report.RenderedPages++ }
	bs := models.NewBuildState(g, nil, report)
	// Compute configuration snapshot hash early; stages.StageGenerateConfig will backfill for other paths.
	bs.Pipeline.ConfigHash = g.ComputeConfigHash()
	report.ConfigHash = bs.Pipeline.ConfigHash

	bs.Git.Repositories = repositories
	bs.Git.WorkspaceDir = filepath.Clean(workspaceDir)

	// Step 2.5: Expand repositories with versioning if enabled
	if g.config.Versioning != nil && !g.config.Versioning.DefaultBranchOnly {
		// Create Git client for version discovery (uses standard workspace)
		gitClient := git.NewClient(bs.Git.WorkspaceDir)
		expanded, err := versioning.ExpandRepositoriesWithVersions(gitClient, g.config)
		if err != nil {
			slog.Warn("Failed to expand repositories with versions, using original list", "error", err)
		} else {
			bs.Git.Repositories = expanded
			slog.Info("Using expanded repository list with versions", "count", len(expanded))
		}
	}

	// Ensure per-repository state exists before stages that attempt to persist metadata
	// (doc counts/hashes) run. This is especially important for discovery-triggered builds
	// where the daemon may not have pre-initialized repository state entries.
	if initializer, ok := any(g.stateManager).(interface {
		EnsureRepositoryState(url, name, branch string)
	}); ok {
		for i := range bs.Git.Repositories {
			r := &bs.Git.Repositories[i]
			initializer.EnsureRepositoryState(r.URL, r.Name, r.Branch)
		}
	}

	pipeline := buildPipeline(true)
	if err := stages.RunStages(ctx, bs, pipeline); err != nil {
		// derive outcome even on error for observability; cleanup staging
		report.DeriveOutcome()
		report.Finish()
		g.abortStaging()
		return report, err
	}
	// Record the commit each repository was built at. The next build reads
	// it back to decide whether anything changed, so it must be persisted
	// here — where the repositories and their commits are both known.
	g.persistRepoCommits(bs)
	// IMPORTANT: stages.RunStages may return nil after an early skip (e.g. no repo
	// HEAD changes and existing output is valid). In that case, we must not promote
	// the staging directory, otherwise we could replace a valid site with an empty
	// scaffold and cause the daemon to start serving 404s.
	if report.SkipReason == skipReasonNoChanges {
		g.abortStaging()
		// The report of a build that was skipped holds no files: the
		// build did not discover any. The next build reads the report to
		// decide whether it may skip, and a report without files is read
		// as "the site is empty" - so every second build would be a
		// full build. The numbers of the last build that ran are kept.
		if prev, ok := g.readPreviousBuildReport(); ok {
			report.Files = prev.Files
			report.Repositories = prev.Repositories
		}
		// best-effort: persist updated report into existing output dir
		if err := report.Persist(g.outputDir); err != nil {
			slog.Warn("Failed to persist build report", "error", err)
		}
		if g.recorder != nil {
			g.recorder.ObserveBuildDuration(report.End.Sub(report.Start))
			g.recorder.IncBuildOutcome(metrics.BuildOutcomeLabel(report.Outcome))
		}
		return report, nil
	}
	// A repository that could not be cloned would take its pages out of
	// the site. The site of the last build still has them, so it stays
	// published and the build fails: the daemon retries, and the
	// operator sees why. When there is no site yet, the build fails as
	// well: a portal without one of its repositories is worse than no
	// portal, and the daemon serves the page that says that the site is
	// being prepared.
	if report.FailedRepositories > 0 {
		g.abortStaging()
		// The report of the build is written, and it says that the
		// build failed: the next build reads it to decide whether it
		// may skip. A report of a build that did not publish must not
		// let the next build skip - the site is behind the workspace,
		// and only a build that runs publishes it.
		report.DeriveOutcome()
		report.Finish()
		if err := report.Persist(g.outputDir); err != nil {
			slog.Warn("Failed to persist build report", "error", err)
		}
		return report, fmt.Errorf("%d of %d repositories could not be cloned; the site was not published",
			report.FailedRepositories, report.Repositories+report.FailedRepositories)
	}
	// Stage durations already written directly to report.
	report.DeriveOutcome()
	report.Finish()
	if err := g.finalizeStaging(); err != nil {
		return report, fmt.Errorf("finalize staging: %w", err)
	}
	if err := report.Persist(g.outputDir); err != nil {
		slog.Warn("Failed to persist build report", "error", err)
	}
	if g.recorder != nil {
		g.recorder.ObserveBuildDuration(report.End.Sub(report.Start))
		g.recorder.IncBuildOutcome(metrics.BuildOutcomeLabel(report.Outcome))
	}
	return report, nil
}

// ComputeConfigHash now delegates to the configuration Snapshot() which produces a
// normalized, stable hash over build-affecting fields. This replaces the previous
// ad-hoc hashing logic to ensure a single source of truth for incremental decisions.
func (g *Generator) ComputeConfigHash() string {
	if g == nil || g.config == nil {
		return ""
	}
	return g.config.Snapshot()
}

// ComputeConfigHashForPersistence exposes the internal config hash used for incremental change detection
// without exporting lower-level implementation details.
func (g *Generator) ComputeConfigHashForPersistence() string { return g.ComputeConfigHash() }

// Config returns the generator configuration.
func (g *Generator) Config() *config.Config         { return g.config }
func (g *Generator) ExistingSiteValidForSkip() bool { return g.existingSiteValidForSkip() }

// SkipIfUnchanged reports whether a build may be skipped when nothing moved.
// The configuration decides: with skip_if_unchanged off, every build runs.
func (g *Generator) SkipIfUnchanged() bool {
	return g.config != nil && g.config.Build.SkipIfUnchanged
}

func (g *Generator) OutputDir() string                            { return g.outputDir }
func (g *Generator) StageDir() string                             { return g.stageDir }
func (g *Generator) Recorder() metrics.Recorder                   { return g.recorder }
func (g *Generator) StateManager() state.RepositoryMetadataWriter { return g.stateManager }
func (g *Generator) Observer() models.BuildObserver               { return g.observer }
func (g *Generator) Renderer() models.Renderer                    { return g.renderer }

func (g *Generator) WithRenderer(r models.Renderer) *Generator {
	if r != nil {
		g.renderer = r
	}
	return g
}

// WithDocumentReady installs a callback invoked once per non-generated
// document immediately after it has been written to the Hugo content
// directory. The callback receives the raw markdown bytes (including YAML
// frontmatter) and the Hugo content path. Pass nil to disable.
//
// This is the daemon-mode hook for pushing document content to a downstream
// consumer such as ragabast for async ingest. The callback MUST be
// non-blocking — copy content if you need to retain it past the call.
func (g *Generator) WithDocumentReady(fn func(content []byte, path string)) *Generator {
	g.onDocumentReady = fn
	return g
}
