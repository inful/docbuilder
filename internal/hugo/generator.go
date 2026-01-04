package hugo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// Generator handles Hugo site generation with Relearn theme.
type Generator struct {
	config    *config.Config
	outputDir string // final output dir
	stageDir  string // ephemeral staging dir for current build
	// optional instrumentation callbacks (not exported)
	onPageRendered func()
	recorder       metrics.Recorder
	observer       BuildObserver // high-level observer (decouples metrics recorder)
	renderer       Renderer      // pluggable renderer abstraction (defaults to BinaryRenderer)
	// editLinkResolver centralizes per-page edit link resolution
	editLinkResolver *EditLinkResolver
	// indexTemplateUsage captures which index template (main/repository/section) source was used
	indexTemplateUsage map[string]IndexTemplateInfo
	// stateManager (optional) allows stages to persist per-repo metadata (doc counts, hashes, commits) without daemon-specific code.
	stateManager state.RepositoryMetadataWriter
	// keepStaging preserves staging directory on failure for debugging (set via WithKeepStaging)
	keepStaging bool
}

// NewGenerator creates a new Hugo site generator.
func NewGenerator(cfg *config.Config, outputDir string) *Generator {
	g := &Generator{config: cfg, outputDir: filepath.Clean(outputDir), recorder: metrics.NoopRecorder{}, indexTemplateUsage: make(map[string]IndexTemplateInfo)}
	// Renderer defaults to nil; stageRunHugo will use BinaryRenderer when needed.
	// Use WithRenderer to inject custom/test renderers.
	// Default observer bridges to recorder until dedicated observers added.
	g.observer = recorderObserver{recorder: g.recorder}
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
	reportPath := filepath.Join(g.outputDir, "build-report.json")
	if fi, err := os.Stat(reportPath); err != nil || fi.IsDir() {
		return false
	}
	publicDir := filepath.Join(g.outputDir, "public")
	if fi, err := os.Stat(publicDir); err != nil || !fi.IsDir() {
		return false
	}
	if entries, err := os.ReadDir(publicDir); err != nil || len(entries) == 0 {
		return false
	}
	contentDir := filepath.Join(g.outputDir, "content")
	if fi, err := os.Stat(contentDir); err != nil || !fi.IsDir() {
		return false
	}
	found := false
	if werr := filepath.WalkDir(contentDir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if found {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			found = true
		}
		return nil
	}); werr != nil {
		return false
	}
	return found
}

// Config exposes the underlying configuration (read-only usage by themes).
func (g *Generator) Config() *config.Config { return g.config }

// SetRecorder injects a metrics recorder (optional). Returns the generator for chaining.
func (g *Generator) SetRecorder(r metrics.Recorder) *Generator {
	if r == nil {
		g.recorder = metrics.NoopRecorder{}
		g.observer = recorderObserver{recorder: g.recorder}
		return g
	}
	g.recorder = r
	g.observer = recorderObserver{recorder: r}
	return g
}

// WithObserver overrides the BuildObserver (takes precedence over internal recorder adapter).
func (g *Generator) WithObserver(o BuildObserver) *Generator {
	if o != nil {
		g.observer = o
	}
	return g
}

// GenerateSite creates a complete Hugo site from discovered documentation.
func (g *Generator) GenerateSite(docFiles []docs.DocFile) error {
	_, err := g.GenerateSiteWithReport(docFiles)
	return err
}

// GenerateSiteWithReport performs site generation (background context) and returns a BuildReport with metrics.
// Prefer GenerateSiteWithReportContext when you have a caller context supporting cancellation/timeouts.
func (g *Generator) GenerateSiteWithReport(docFiles []docs.DocFile) (*BuildReport, error) {
	return g.GenerateSiteWithReportContext(context.Background(), docFiles)
}

// GenerateSiteWithReportContext performs site generation honoring the provided context for cancellation.
func (g *Generator) GenerateSiteWithReportContext(ctx context.Context, docFiles []docs.DocFile) (*BuildReport, error) {
	slog.Info("Starting Hugo site generation", slog.String("output", g.outputDir), slog.Int("files", len(docFiles)))
	if err := g.beginStaging(); err != nil {
		return nil, err
	}
	repoSet := map[string]struct{}{}
	for i := range docFiles {
		f := &docFiles[i]
		repoSet[f.Repository] = struct{}{}
	}
	report := newBuildReport(ctx, len(repoSet), len(docFiles))
	// Populate observability enrichment fields
	report.PipelineVersion = 1
	report.EffectiveRenderMode = string(config.ResolveEffectiveRenderMode(g.config))
	// Direct generation path bypasses clone stage entirely.
	report.CloneStageSkipped = true
	// instrumentation hook to count rendered pages
	g.onPageRendered = func() { report.RenderedPages++ }

	bs := newBuildState(g, docFiles, report)

	stages := NewPipeline().
		Add(StagePrepareOutput, stagePrepareOutput).
		Add(StageGenerateConfig, stageGenerateConfig).
		Add(StageLayouts, stageLayouts).
		Add(StageCopyContent, stageCopyContent).
		Add(StageIndexes, stageIndexes).
		Add(StageRunHugo, stageRunHugo).
		Add(StagePostProcess, stagePostProcess).
		Build()

	if err := runStages(ctx, bs, stages); err != nil {
		// cleanup staging dir on failure
		g.abortStaging()
		// If clone stage executed (presence of durations entry) flip flag.
		if _, ok := report.StageDurations[string(StageCloneRepos)]; ok {
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
		// Simple insertion sort to avoid importing sort (small slice typical for tests)
		for i := 1; i < len(paths); i++ {
			j := i
			for j > 0 && paths[j-1] > paths[j] {
				paths[j-1], paths[j] = paths[j], paths[j-1]
				j--
			}
		}
		h := sha256.New()
		for _, p := range paths {
			_, _ = h.Write([]byte(p))
			_, _ = h.Write([]byte{0})
		}
		report.DocFilesHash = hex.EncodeToString(h.Sum(nil))
	}

	// Stage durations already written directly to report.

	report.deriveOutcome()
	report.finish()
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

	if _, ok := report.StageDurations[string(StageCloneRepos)]; ok {
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
func (g *Generator) GenerateFullSite(ctx context.Context, repositories []config.Repository, workspaceDir string) (*BuildReport, error) {
	report := newBuildReport(ctx, 0, 0) // counts filled after discovery
	report.PipelineVersion = 1
	report.EffectiveRenderMode = string(config.ResolveEffectiveRenderMode(g.config))
	// By default full site path includes clone stage; mark skipped=false (may stay false)
	report.CloneStageSkipped = false
	if err := g.beginStaging(); err != nil {
		return nil, err
	}
	g.onPageRendered = func() { report.RenderedPages++ }
	bs := newBuildState(g, nil, report)
	// Compute configuration snapshot hash early; stageGenerateConfig will backfill for other paths.
	bs.Pipeline.ConfigHash = g.computeConfigHash()
	report.ConfigHash = bs.Pipeline.ConfigHash

	bs.Git.Repositories = repositories
	bs.Git.WorkspaceDir = filepath.Clean(workspaceDir)

	stages := NewPipeline().
		Add(StagePrepareOutput, stagePrepareOutput).
		Add(StageCloneRepos, stageCloneRepos).
		Add(StageDiscoverDocs, stageDiscoverDocs).
		Add(StageGenerateConfig, stageGenerateConfig).
		Add(StageLayouts, stageLayouts).
		Add(StageCopyContent, stageCopyContent).
		Add(StageIndexes, stageIndexes).
		Add(StageRunHugo, stageRunHugo).
		Add(StagePostProcess, stagePostProcess).
		Build()
	if err := runStages(ctx, bs, stages); err != nil {
		// derive outcome even on error for observability; cleanup staging
		report.deriveOutcome()
		report.finish()
		g.abortStaging()
		return report, err
	}
	// Stage durations already written directly to report.
	report.deriveOutcome()
	report.finish()
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

// computeConfigHash now delegates to the configuration Snapshot() which produces a
// normalized, stable hash over build-affecting fields. This replaces the previous
// ad-hoc hashing logic to ensure a single source of truth for incremental decisions.
func (g *Generator) computeConfigHash() string {
	if g == nil || g.config == nil {
		return ""
	}
	return g.config.Snapshot()
}

// ComputeConfigHashForPersistence exposes the internal config hash used for incremental change detection
// without exporting lower-level implementation details.
func (g *Generator) ComputeConfigHashForPersistence() string { return g.computeConfigHash() }
