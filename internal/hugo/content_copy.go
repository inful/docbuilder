package hugo

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	tr "git.home.luguber.info/inful/docbuilder/internal/hugo/transforms"
)

// copyContentFiles copies documentation files to Hugo content directory
func (g *Generator) copyContentFiles(ctx context.Context, docFiles []docs.DocFile) error {
	// NOTE: Registry-based pipeline scaffolding exists (internal/hugo/transforms) but
	// is currently disabled until a proper PageShim adapter is implemented. Always
	// use legacy explicit pipeline for now to guarantee identical behavior.
	useRegistry := false
	var regs []tr.Transformer
	for _, file := range docFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := file.LoadContent(); err != nil {
			return fmt.Errorf("failed to load content for %s: %w", file.Path, err)
		}
		p := &Page{File: file, Raw: file.Content, Content: string(file.Content), OriginalFrontMatter: nil, Patches: nil}
		if useRegistry {
			for _, rt := range regs { // ordered by priority already
				if err := rt.Transform(p); err != nil {
					return fmt.Errorf("transform %s failed for %s: %w", rt.Name(), file.Path, err)
				}
			}
		} else {
			pipeline := NewTransformerPipeline(
				&FrontMatterParser{},
				&FrontMatterBuilder{ConfigProvider: func() *Generator { return g }},
				&EditLinkInjector{ConfigProvider: func() *Generator { return g }},
				&MergeFrontMatterTransformer{},
				&RelativeLinkRewriter{},
				&FinalFrontMatterSerializer{},
			)
			if err := pipeline.Run(p); err != nil {
				return fmt.Errorf("pipeline failed for %s: %w", file.Path, err)
			}
		}
		outputPath := filepath.Join(g.buildRoot(), file.GetHugoPath())
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", outputPath, err)
		}
		if err := os.WriteFile(outputPath, p.Raw, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", outputPath, err)
		}
		slog.Debug("Copied content file", slog.String("source", file.RelativePath), slog.String("destination", file.GetHugoPath()))
		// We cannot directly access BuildReport here cleanly without refactor; use optional callback if set.
		if g.onPageRendered != nil {
			g.onPageRendered()
		}
	}
	slog.Info("Copied all content files", slog.Int("count", len(docFiles)))
	return nil
}

// deprecated processMarkdownFile removed (unused)
