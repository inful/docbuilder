package hugo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
	tr "git.home.luguber.info/inful/docbuilder/internal/hugo/transforms"
	"gopkg.in/yaml.v3"
)

// copyContentFiles copies documentation files to Hugo content directory.
// Supports both legacy registry-based pipeline and new fixed pipeline.
func (g *Generator) copyContentFiles(ctx context.Context, docFiles []docs.DocFile) error {
	// Check for new pipeline flag (environment variable for testing)
	useNewPipeline := os.Getenv("DOCBUILDER_NEW_PIPELINE") == "1"

	if useNewPipeline {
		slog.Info("Using NEW fixed transform pipeline (ADR-003)")
		return g.copyContentFilesPipeline(ctx, docFiles)
	}

	slog.Debug("Using legacy registry-based transform pipeline")

	// Legacy pipeline implementation follows...
	// Validate transform pipeline before execution
	if err := g.ValidateTransformPipeline(); err != nil {
		return fmt.Errorf("%w: %w", herrors.ErrContentTransformFailed, err)
	}

	// Build transform pipeline
	transformList, err := tr.List()
	if err != nil {
		return fmt.Errorf("%w: failed to build transform pipeline: %w", herrors.ErrContentTransformFailed, err)
	}
	slog.Debug("Using dependency-based transform pipeline", slog.Int("count", len(transformList)))

	if len(transformList) == 0 {
		return fmt.Errorf("%w: no transforms available", herrors.ErrContentTransformFailed)
	}
	// Use index-based iteration to enable mutating DocFile.TransformedBytes
	for i := range docFiles {
		file := &docFiles[i]
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Handle assets differently - just copy them without processing
		if file.IsAsset {
			if err := g.copyAssetFile(*file); err != nil {
				return fmt.Errorf("failed to copy asset %s: %w", file.Path, err)
			}
			continue
		}

		// Process markdown files with transforms
		if err := file.LoadContent(); err != nil {
			return fmt.Errorf("failed to load content for %s: %w", file.Path, err)
		}
		p := &Page{File: *file, Raw: file.Content, Content: string(file.Content), OriginalFrontMatter: nil, Patches: nil}
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
				Doc:                 *file,
				Content:             p.Content,
				OriginalFrontMatter: p.OriginalFrontMatter,
				HadFrontMatter:      p.HadFrontMatter,
				SyncOriginal: func(fm map[string]any, had bool) {
					p.OriginalFrontMatter = fm
					p.HadFrontMatter = had
				},
				BackingAddPatch: func(pt fmcore.FrontMatterPatch) { p.Patches = append(p.Patches, pt) },
				ApplyPatches:    func() { p.applyPatches() },
				RewriteLinks: func(s string) string {
					isIndex := strings.ToLower(file.Name) == "index" || strings.ToLower(file.Name) == "readme"
					return RewriteRelativeMarkdownLinks(s, file.Repository, file.Forge, isIndex)
				},
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
			for _, transform := range transformList { // ordered by dependencies
				name := transform.Name()

				// Apply filtering if configured
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
				err := transform.Transform(shim)
				dur := time.Since(start)
				success := err == nil
				if g.recorder != nil {
					g.recorder.ObserveContentTransformDuration(name, dur, success)
				}
				if err != nil {
					if g.recorder != nil {
						g.recorder.IncContentTransformFailure(name)
					}
					return fmt.Errorf("%w: %s failed for %s: %w", herrors.ErrContentTransformFailed, name, file.Path, err)
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

		// Capture transformed content for use by index generation stage
		file.TransformedBytes = p.Raw
		slog.Debug("Captured transformed content",
			slog.String("file", file.RelativePath),
			slog.Int("bytes", len(p.Raw)))

		outputPath := filepath.Join(g.buildRoot(), file.GetHugoPath())
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
			return fmt.Errorf("%w: failed to create directory for %s: %w", herrors.ErrContentWriteFailed, outputPath, err)
		}
		// #nosec G306 -- content files are public documentation
		if err := os.WriteFile(outputPath, p.Raw, 0o644); err != nil {
			return fmt.Errorf("%w: failed to write file %s: %w", herrors.ErrContentWriteFailed, outputPath, err)
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

// copyAssetFile copies an asset file (image, etc.) to Hugo content directory without processing
func (g *Generator) copyAssetFile(file docs.DocFile) error {
	// Read the asset file
	content, err := os.ReadFile(file.Path)
	if err != nil {
		return fmt.Errorf("%w: failed to read asset %s: %w", herrors.ErrContentWriteFailed, file.Path, err)
	}

	// Calculate output path - assets go in same location as markdown files
	outputPath := filepath.Join(g.buildRoot(), file.GetHugoPath())

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return fmt.Errorf("%w: failed to create directory for %s: %w", herrors.ErrContentWriteFailed, outputPath, err)
	}

	// Copy asset file as-is
	// #nosec G306 -- asset files are public documentation resources
	if err := os.WriteFile(outputPath, content, 0o644); err != nil {
		return fmt.Errorf("%w: failed to write asset %s: %w", herrors.ErrContentWriteFailed, outputPath, err)
	}

	slog.Debug("Copied asset file",
		slog.String("source", file.RelativePath),
		slog.String("destination", file.GetHugoPath()),
		slog.String("type", file.Extension))

	return nil
}

// Note: legacy processMarkdownFile helper was removed as unused.
