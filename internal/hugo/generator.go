package hugo

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
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
}

// ThemeFeatures describes capability flags & module path for the selected theme.
type ThemeFeatures struct {
	Name                   config.Theme
	UsesModules            bool
	ModulePath             string
	EnableMathPassthrough  bool
	EnableOfflineSearchJSON bool
	AutoMainMenu           bool // if true and no explicit menu, we inject default main menu
}

// deriveThemeFeatures inspects configuration and returns normalized feature flags.
func (g *Generator) deriveThemeFeatures() ThemeFeatures {
	t := g.config.Hugo.ThemeType()
	feats := ThemeFeatures{Name: t}
	switch t {
	case config.ThemeHextra:
		feats.UsesModules = true
		feats.ModulePath = "github.com/imfing/hextra"
		feats.EnableMathPassthrough = true
		feats.EnableOfflineSearchJSON = false // Hextra's offline search handled via params; no outputs JSON needed
		feats.AutoMainMenu = true
	case config.ThemeDocsy:
		feats.UsesModules = true
		feats.ModulePath = "github.com/google/docsy"
		feats.EnableMathPassthrough = false
		feats.EnableOfflineSearchJSON = true
		feats.AutoMainMenu = false
	default:
		// unknown/custom theme - no special features
	}
	return feats
}

// NewGenerator creates a new Hugo site generator
func NewGenerator(cfg *config.Config, outputDir string) *Generator {
	return &Generator{config: cfg, outputDir: filepath.Clean(outputDir), recorder: metrics.NoopRecorder{}}
}

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
	// instrumentation hook to count rendered pages
	g.onPageRendered = func() { report.RenderedPages++ }

	bs := newBuildState(g, docFiles, report)

	stages := []StageDef{
		{StagePrepareOutput, stagePrepareOutput},
		{StageGenerateConfig, stageGenerateConfig},
		{StageLayouts, stageLayouts},
		{StageCopyContent, stageCopyContent},
		{StageIndexes, stageIndexes},
		{StageRunHugo, stageRunHugo},
		{StagePostProcess, stagePostProcess},
	}

	if err := runStages(ctx, bs, stages); err != nil {
		// cleanup staging dir on failure
		g.abortStaging()
		return nil, err
	}

	// Stage durations already written directly to report.

	report.deriveOutcome()
	report.finish()
	if err := g.finalizeStaging(); err != nil {
		return nil, fmt.Errorf("finalize staging: %w", err)
	}
	// Persist report (best effort) inside final output directory
	if err := report.Persist(g.outputDir); err != nil {
		slog.Warn("Failed to persist build report", "error", err)
	}
	// record build-level metrics
	if g.recorder != nil {
		g.recorder.ObserveBuildDuration(report.End.Sub(report.Start))
		// convert typed outcome; if unset fall back to legacy string
		out := report.OutcomeT
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
	if err := g.beginStaging(); err != nil {
		return nil, err
	}
	g.onPageRendered = func() { report.RenderedPages++ }
	bs := newBuildState(g, nil, report)

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

	stages := []StageDef{
		{StagePrepareOutput, stagePrepareOutput},
		{StageCloneRepos, stageCloneRepos},
		{StageDiscoverDocs, stageDiscoverDocs},
		{StageGenerateConfig, stageGenerateConfig},
		{StageLayouts, stageLayouts},
		{StageCopyContent, stageCopyContent},
		{StageIndexes, stageIndexes},
		{StageRunHugo, stageRunHugo},
		{StagePostProcess, stagePostProcess},
	}
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
		out := report.OutcomeT
		if out == "" {
			out = BuildOutcome(report.Outcome)
		}
		g.recorder.IncBuildOutcome(metrics.BuildOutcomeLabel(out))
	}
	return report, nil
}
