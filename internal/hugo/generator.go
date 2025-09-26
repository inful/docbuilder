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
	slog.Info("Starting Hugo site generation", "output", g.outputDir, "files", len(docFiles))
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

	stages := []struct {
		name string
		fn   Stage
	}{
		{"prepare_output", stagePrepareOutput},
		{"generate_config", stageGenerateConfig},
		{"layouts", stageLayouts},
		{"copy_content", stageCopyContent},
		{"indexes", stageIndexes},
		{"run_hugo", stageRunHugo},
		{"post_process", stagePostProcess},
	}

	if err := runStages(ctx, bs, stages); err != nil {
		// cleanup staging dir on failure
		g.abortStaging()
		return nil, err
	}

	// transfer timings into report (keep separate to allow future aggregation logic)
	for k, v := range bs.Timings {
		report.StageDurations[k] = v
	}

	report.deriveOutcome()
	report.finish()
	if err := g.finalizeStaging(); err != nil {
		return nil, fmt.Errorf("finalize staging: %w", err)
	}
	// record build-level metrics
	if g.recorder != nil {
		g.recorder.ObserveBuildDuration(report.End.Sub(report.Start))
		g.recorder.IncBuildOutcome(report.Outcome)
	}
	slog.Info("Hugo site generation completed", "output", g.outputDir, "repos", report.Repositories, "files", report.Files, "errors", len(report.Errors))
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

	stages := []struct {
		name string
		fn   Stage
	}{
		{"prepare_output", stagePrepareOutput},
		{"clone_repos", stageCloneRepos},
		{"discover_docs", stageDiscoverDocs},
		{"generate_config", stageGenerateConfig},
		{"layouts", stageLayouts},
		{"copy_content", stageCopyContent},
		{"indexes", stageIndexes},
		{"run_hugo", stageRunHugo},
		{"post_process", stagePostProcess},
	}
	if err := runStages(ctx, bs, stages); err != nil {
		// derive outcome even on error for observability; cleanup staging
		report.deriveOutcome()
		report.finish()
		g.abortStaging()
		return report, err
	}
	for k, v := range bs.Timings {
		report.StageDurations[k] = v
	}
	report.deriveOutcome()
	report.finish()
	if err := g.finalizeStaging(); err != nil {
		return report, fmt.Errorf("finalize staging: %w", err)
	}
	if g.recorder != nil {
		g.recorder.ObserveBuildDuration(report.End.Sub(report.Start))
		g.recorder.IncBuildOutcome(report.Outcome)
	}
	return report, nil
}
