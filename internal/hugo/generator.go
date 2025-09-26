package hugo

import (
	"context"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// Generator handles Hugo site generation
type Generator struct {
	config    *config.Config
	outputDir string
	// optional instrumentation callbacks (not exported)
	onPageRendered func()
}

// NewGenerator creates a new Hugo site generator
func NewGenerator(cfg *config.Config, outputDir string) *Generator {
	return &Generator{
		config:    cfg,
		outputDir: outputDir,
	}
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
		return nil, err
	}

	// transfer timings into report (keep separate to allow future aggregation logic)
	for k, v := range bs.Timings {
		report.StageDurations[k] = v
	}

	report.deriveOutcome()
	report.finish()
	slog.Info("Hugo site generation completed", "output", g.outputDir, "repos", report.Repositories, "files", report.Files, "errors", len(report.Errors))
	return report, nil
}

// (Helper methods split into separate files for maintainability.)
