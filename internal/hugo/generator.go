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
	th "git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
	_ "git.home.luguber.info/inful/docbuilder/internal/hugo/themes/docsy"
	_ "git.home.luguber.info/inful/docbuilder/internal/hugo/themes/hextra"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/repository"
)

// Generator handles Hugo site generation
type Generator struct {
	config    *config.Config
	outputDir string // final output dir
	stageDir  string // ephemeral staging dir for current build
	// optional instrumentation callbacks (not exported)
	onPageRendered func()
	recorder       metrics.Recorder
	// cachedThemeFeatures stores the lazily-computed feature flags for the selected theme
	cachedThemeFeatures *th.ThemeFeatures
	// editLinkResolver centralizes per-page edit link resolution
	editLinkResolver *EditLinkResolver
	// indexTemplateUsage captures which index template (main/repository/section) source was used
	indexTemplateUsage map[string]IndexTemplateInfo
}

// activeTheme returns current registered theme.
func (g *Generator) activeTheme() th.Theme { return th.Get(g.config.Hugo.ThemeType()) }

// deriveThemeFeatures obtains and caches theme features; unknown themes return minimal struct.
func (g *Generator) deriveThemeFeatures() th.ThemeFeatures {
	if g.cachedThemeFeatures != nil {
		return *g.cachedThemeFeatures
	}
	if tt := th.Get(g.config.Hugo.ThemeType()); tt != nil {
		feats := tt.Features()
		g.cachedThemeFeatures = &feats
		return feats
	}
	feats := th.ThemeFeatures{Name: g.config.Hugo.ThemeType()}
	g.cachedThemeFeatures = &feats
	return feats
}

// NewGenerator creates a new Hugo site generator
func NewGenerator(cfg *config.Config, outputDir string) *Generator {
	g := &Generator{config: cfg, outputDir: filepath.Clean(outputDir), recorder: metrics.NoopRecorder{}, indexTemplateUsage: make(map[string]IndexTemplateInfo)}
	// Initialize resolver eagerly (cheap) to simplify call sites.
	g.editLinkResolver = NewEditLinkResolver(cfg)
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
	_ = filepath.WalkDir(contentDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			found = true
		}
		return nil
	})
	return found
}

// Config exposes the underlying configuration (read-only usage by themes).
func (g *Generator) Config() *config.Config { return g.config }

// SetRecorder injects a metrics recorder (optional). Returns the generator for chaining.
func (g *Generator) SetRecorder(r metrics.Recorder) *Generator {
	if r == nil {
		g.recorder = metrics.NoopRecorder{}
		return g
	}
	g.recorder = r
	return g
}

// GenerateSite creates a complete Hugo site from discovered documentation
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
	for _, f := range docFiles {
		repoSet[f.Repository] = struct{}{}
	}
	report := newBuildReport(len(repoSet), len(docFiles))
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

	// Stage durations already written directly to report.

	report.deriveOutcome()
	report.finish()
	if err := g.finalizeStaging(); err != nil {
		return nil, fmt.Errorf("finalize staging: %w", err)
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
		// convert typed outcome; if unset fall back to legacy string
		out := report.Outcome
		if out == "" {
			out = BuildOutcome(report.Outcome)
		}
		g.recorder.IncBuildOutcome(metrics.BuildOutcomeLabel(out))
	}
	slog.Info("Hugo site generation completed", slog.String("output", g.outputDir), slog.Int("repos", report.Repositories), slog.Int("files", report.Files), slog.Int("errors", len(report.Errors)))
	return report, nil
}

// (Helper methods split into separate files for maintainability.)

// GenerateFullSite clones repositories, discovers documentation, then executes the standard generation stages.
// repositories: list of repositories to process. workspaceDir: directory for git operations (created if missing).
func (g *Generator) GenerateFullSite(ctx context.Context, repositories []config.Repository, workspaceDir string) (*BuildReport, error) {
	report := newBuildReport(0, 0) // counts filled after discovery
	// By default full site path includes clone stage; mark skipped=false (may stay false)
	report.CloneStageSkipped = false
	if err := g.beginStaging(); err != nil {
		return nil, err
	}
	g.onPageRendered = func() { report.RenderedPages++ }
	bs := newBuildState(g, nil, report)
	// Compute a minimal config hash for change detection (theme + baseURL + params hash length) - extensible.
	bs.ConfigHash = g.computeConfigHash()

	// Apply repository filter if config has patterns (future extension: config fields).
	// Placeholder: look for params under g.config.Hugo.Params["filter"] map with keys include/exclude.
	if g.config != nil && g.config.Hugo.Params != nil {
		if raw, ok := g.config.Hugo.Params["filter"]; ok {
			if m, ok2 := raw.(map[string]any); ok2 {
				var includes, excludes []string
				if v, ok := m["include"].([]any); ok {
					for _, it := range v {
						if s, ok := it.(string); ok {
							includes = append(includes, s)
						}
					}
				}
				if v, ok := m["exclude"].([]any); ok {
					for _, it := range v {
						if s, ok := it.(string); ok {
							excludes = append(excludes, s)
						}
					}
				}
				if len(includes) > 0 || len(excludes) > 0 {
					if f, err := repository.NewFilter(includes, excludes); err == nil {
						filtered := make([]config.Repository, 0, len(repositories))
						for _, r := range repositories {
							if ok, _ := f.Include(r); ok {
								filtered = append(filtered, r)
							} else {
								report.SkippedRepositories++
							}
						}
						repositories = filtered
					}
				}
			}
		}
	}

	bs.Repositories = repositories
	bs.WorkspaceDir = filepath.Clean(workspaceDir)

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
		out := report.Outcome
		if out == "" {
			out = BuildOutcome(report.Outcome)
		}
		g.recorder.IncBuildOutcome(metrics.BuildOutcomeLabel(out))
	}
	return report, nil
}

// computeConfigHash generates a stable fingerprint of config fields that should trigger a rebuild
// even when repository commits have not changed. This is intentionally narrow to avoid false positives
// and can be expanded as new features require (e.g. menu configuration, output settings).
func (g *Generator) computeConfigHash() string {
	if g == nil || g.config == nil {
		return ""
	}
	h := sha256.New()
	cfg := g.config
	// Include key high-impact fields.
	h.Write([]byte(cfg.Hugo.Title))
	h.Write([]byte(cfg.Hugo.Theme))
	h.Write([]byte(cfg.Hugo.BaseURL))
	// Very lightweight params inclusion: just the keys in deterministic order + their string forms.
	if cfg.Hugo.Params != nil {
		// Collect keys
		keys := make([]string, 0, len(cfg.Hugo.Params))
		for k := range cfg.Hugo.Params {
			keys = append(keys, k)
		}
		// Simple insertion sort (small map expected) to avoid pulling in sort import if not already.
		for i := 1; i < len(keys); i++ {
			j := i
			for j > 0 && keys[j-1] > keys[j] {
				keys[j-1], keys[j] = keys[j], keys[j-1]
				j--
			}
		}
		for _, k := range keys {
			h.Write([]byte(k))
			if v := cfg.Hugo.Params[k]; v != nil {
				h.Write([]byte(fmt.Sprintf("%v", v)))
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ComputeConfigHashForPersistence exposes the internal config hash used for incremental change detection
// without exporting lower-level implementation details.
func (g *Generator) ComputeConfigHashForPersistence() string { return g.computeConfigHash() }
