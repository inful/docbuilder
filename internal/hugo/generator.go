package hugo

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// Generator handles Hugo site generation
type Generator struct {
	config    *config.Config
	outputDir string
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

// GenerateSiteWithReport performs site generation and returns a BuildReport with metrics.
func (g *Generator) GenerateSiteWithReport(docFiles []docs.DocFile) (*BuildReport, error) {
	slog.Info("Starting Hugo site generation", "output", g.outputDir, "files", len(docFiles))
	repoSet := map[string]struct{}{}
	for _, f := range docFiles {
		repoSet[f.Repository] = struct{}{}
	}
	report := newBuildReport(len(repoSet), len(docFiles))

	addErr := func(err error) { // non-fatal phase errors
		if err != nil {
			report.Errors = append(report.Errors, err)
			slog.Warn("Generation phase error", "error", err)
		}
	}

	if err := g.createHugoStructure(); err != nil {
		return nil, fmt.Errorf("failed to create Hugo structure: %w", err)
	}
	if err := g.generateHugoConfig(); err != nil {
		return nil, fmt.Errorf("failed to generate Hugo config: %w", err)
	}
	if g.config.Hugo.Theme == "" { // basic fallback layouts
		if err := g.generateBasicLayouts(); err != nil {
			return nil, fmt.Errorf("failed to generate layouts: %w", err)
		}
	}
	if err := g.copyContentFiles(docFiles); err != nil {
		return nil, fmt.Errorf("failed to copy content files: %w", err)
	}
	if err := g.generateIndexPages(docFiles); err != nil {
		return nil, fmt.Errorf("failed to generate index pages: %w", err)
	}
	if shouldRunHugo() { // optional external hugo binary
		if err := g.runHugoBuild(); err != nil {
			addErr(fmt.Errorf("hugo build failed: %w", err))
		} else {
			slog.Info("Hugo static site build completed", "public", filepath.Join(g.outputDir, "public"))
		}
	} else {
		slog.Info("Skipping Hugo executable run (set DOCBUILDER_RUN_HUGO=1 to enable)")
	}
	report.finish()
	slog.Info("Hugo site generation completed", "output", g.outputDir, "repos", report.Repositories, "files", report.Files, "errors", len(report.Errors))
	return report, nil
}

// (Helper methods split into separate files for maintainability.)
