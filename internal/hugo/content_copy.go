package hugo

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// copyContentFiles copies documentation files to Hugo content directory
func (g *Generator) copyContentFiles(ctx context.Context, docFiles []docs.DocFile) error {
	pipeline := NewTransformerPipeline(
		&FrontMatterParser{},
		&FrontMatterBuilder{ConfigProvider: func() *Generator { return g }},
		&EditLinkInjector{ConfigProvider: func() *Generator { return g }},
		&MergeFrontMatterTransformer{}, // produce merged view (future transformers could rely on it)
		&RelativeLinkRewriter{},
		&FinalFrontMatterSerializer{},
	)
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
		if err := pipeline.Run(p); err != nil {
			return fmt.Errorf("pipeline failed for %s: %w", file.Path, err)
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
