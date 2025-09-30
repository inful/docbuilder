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
	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

// copyContentFiles copies documentation files to Hugo content directory
func (g *Generator) copyContentFiles(ctx context.Context, docFiles []docs.DocFile) error {
	regs := tr.List()
	if len(regs) == 0 {
		return fmt.Errorf("no content transforms registered")
	}
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
		{
			// Prepare transform filtering
			var enableSet, disableSet map[string]struct{}
			if g.config != nil && g.config.Hugo.Transforms != nil {
				if len(g.config.Hugo.Transforms.Enable) > 0 {
					enableSet = map[string]struct{}{}
					for _, n := range g.config.Hugo.Transforms.Enable {
						enableSet[n] = struct{}{}
					}
				}
				if len(g.config.Hugo.Transforms.Disable) > 0 {
					disableSet = map[string]struct{}{}
					for _, n := range g.config.Hugo.Transforms.Disable {
						disableSet[n] = struct{}{}
					}
				}
			}
			// Build adapter shim (two-phase to allow Serialize closure to reference shim)
			shim := &tr.PageShim{
				FilePath:            file.RelativePath,
				DocFile:             func() struct{ Repository, Forge, Section, Name string; Metadata map[string]any } {
					md := make(map[string]any, len(file.Metadata))
					for k, v := range file.Metadata { md[k] = v }
					return struct{ Repository, Forge, Section, Name string; Metadata map[string]any }{Repository: file.Repository, Forge: file.Forge, Section: file.Section, Name: file.Name, Metadata: md}
				}(),
				Content:             p.Content,
				OriginalFrontMatter: p.OriginalFrontMatter,
				HadFrontMatter:      p.HadFrontMatter,
				// SyncOriginal allows the parser (registry transform) to push parsed front matter
				// into the real Page before subsequent transforms (like the builder) run. Without
				// this, builder sees nil Existing FM and overwrites user-provided keys (bug found
				// by TestPipeline_Order losing 'custom: val').
				SyncOriginal: func(fm map[string]any, had bool) {
					p.OriginalFrontMatter = fm
					p.HadFrontMatter = had
				},
				// Build front matter using existing helper
				BuildFrontMatter: func(now time.Time) {
					built := BuildFrontMatter(FrontMatterInput{File: p.File, Existing: p.OriginalFrontMatter, Config: g.config, Now: now})
					p.Patches = append(p.Patches, fmcore.FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 50, Data: built})
				},
				InjectEditLink: func() {
					if p.OriginalFrontMatter != nil {
						if _, ok := p.OriginalFrontMatter["editURL"]; ok {
							return
						}
					}
					for _, patch := range p.Patches {
						if patch.Data != nil {
							if _, ok := patch.Data["editURL"]; ok {
								return
							}
						}
					}
					if g.editLinkResolver == nil {
						return
					}
					val := g.editLinkResolver.Resolve(p.File)
					if val == "" {
						return
					}
					p.Patches = append(p.Patches, fmcore.FrontMatterPatch{Source: "edit_link", Mode: MergeSetIfMissing, Priority: 60, Data: map[string]any{"editURL": val}})
				},
				ApplyPatches: func() { p.applyPatches() },
				RewriteLinks: func(s string) string { return RewriteRelativeMarkdownLinks(s) },
			}
			shim.SerializeFn = func() error {
				if p.MergedFrontMatter == nil {
					p.applyPatches()
				}
				p.Content = shim.Content
				fm := p.MergedFrontMatter
				if fm == nil {
					fm = map[string]any{}
				}
				fmData, err := yaml.Marshal(fm)
				if err != nil {
					return err
				}
				combined := fmt.Sprintf("---\n%s---\n%s", string(fmData), p.Content)
				p.Raw = []byte(combined)
				return nil
			}
			for _, rt := range regs { // ordered
				name := rt.Name()
				if disableSet != nil {
					if _, blocked := disableSet[name]; blocked {
						continue
					}
				}
				if enableSet != nil {
					if _, ok := enableSet[name]; !ok {
						continue
					}
				}
				start := time.Now()
				err := rt.Transform(shim)
				dur := time.Since(start)
				success := err == nil
				if g.recorder != nil {
					g.recorder.ObserveContentTransformDuration(name, dur, success)
				}
				if err != nil {
					if g.recorder != nil {
						g.recorder.IncContentTransformFailure(name)
					}
					return fmt.Errorf("transform %s failed for %s: %w", rt.Name(), file.Path, err)
				}
			}
			// Sync back mutated fields
			p.Content = shim.Content
			p.OriginalFrontMatter = shim.OriginalFrontMatter
			p.HadFrontMatter = shim.HadFrontMatter
			// Raw set in Serialize
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
