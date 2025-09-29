package hugo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	tr "git.home.luguber.info/inful/docbuilder/internal/hugo/transforms"
	"gopkg.in/yaml.v3"
)

// copyContentFiles copies documentation files to Hugo content directory
func (g *Generator) copyContentFiles(ctx context.Context, docFiles []docs.DocFile) error {
	regs := tr.List()
	useRegistry := len(regs) > 0
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
			// Build adapter shim (two-phase to allow Serialize closure to reference shim)
			shim := &tr.PageShim{
				FilePath: file.RelativePath,
				Content:  p.Content,
				OriginalFrontMatter: p.OriginalFrontMatter,
				HadFrontMatter:      p.HadFrontMatter,
				// Build front matter using existing helper
				BuildFrontMatter: func(now time.Time) {
					built := BuildFrontMatter(FrontMatterInput{File: p.File, Existing: p.OriginalFrontMatter, Config: g.config, Now: now})
					p.Patches = append(p.Patches, FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 50, Data: built})
				},
				InjectEditLink: func() {
					if p.OriginalFrontMatter != nil {
						if _, ok := p.OriginalFrontMatter["editURL"]; ok { return }
					}
					for _, patch := range p.Patches { if patch.Data != nil { if _, ok := patch.Data["editURL"]; ok { return } } }
					if g.editLinkResolver == nil { return }
					val := g.editLinkResolver.Resolve(p.File)
					if val == "" { return }
					p.Patches = append(p.Patches, FrontMatterPatch{Source: "edit_link", Mode: MergeSetIfMissing, Priority: 60, Data: map[string]any{"editURL": val}})
				},
				ApplyPatches: func() { p.applyPatches() },
				RewriteLinks: func(s string) string { return RewriteRelativeMarkdownLinks(s) },
			}
			shim.Serialize = func() error {
				if p.MergedFrontMatter == nil { p.applyPatches() }
				p.Content = shim.Content
				fm := p.MergedFrontMatter
				if fm == nil { fm = map[string]any{} }
				fmData, err := yaml.Marshal(fm)
				if err != nil { return err }
				combined := fmt.Sprintf("---\n%s---\n%s", string(fmData), p.Content)
				p.Raw = []byte(combined)
				return nil
			}
			for _, rt := range regs { // ordered
				if err := rt.Transform(shim); err != nil {
					return fmt.Errorf("transform %s failed for %s: %w", rt.Name(), file.Path, err)
				}
			}
			// Sync back mutated fields
			p.Content = shim.Content
			p.OriginalFrontMatter = shim.OriginalFrontMatter
			p.HadFrontMatter = shim.HadFrontMatter
			// Raw set in Serialize
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
		// record hash of raw for potential future integrity verification (not persisted yet)
		_ = sha256.Sum256(p.Raw)
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
