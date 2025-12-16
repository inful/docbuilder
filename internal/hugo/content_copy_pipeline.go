package hugo

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/pipeline"
)

// copyContentFilesPipeline copies documentation files using the new fixed transform pipeline.
// This is the new implementation that replaces the registry-based transform system.
func (g *Generator) copyContentFilesPipeline(ctx context.Context, docFiles []docs.DocFile) error {
	slog.Info("Using new fixed transform pipeline for content processing")

	// Separate markdown files from assets
	var markdownFiles []docs.DocFile
	var assetFiles []docs.DocFile

	for _, file := range docFiles {
		if file.IsAsset {
			assetFiles = append(assetFiles, file)
		} else {
			markdownFiles = append(markdownFiles, file)
		}
	}

	// Process assets first (simple copy)
	for _, file := range assetFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := g.copyAssetFile(file); err != nil {
			return fmt.Errorf("failed to copy asset %s: %w", file.Path, err)
		}
	}

	// Convert DocFiles to pipeline Documents
	discovered := make([]*pipeline.Document, 0, len(markdownFiles))
	for _, file := range markdownFiles {
		// Load content
		if err := file.LoadContent(); err != nil {
			return fmt.Errorf("%w: failed to load content for %s: %w",
				herrors.ErrContentTransformFailed, file.Path, err)
		}

		// Convert to pipeline Document
		doc := pipeline.NewDocumentFromDocFile(file)
		discovered = append(discovered, doc)
	}

	slog.Info("Converted discovered files to pipeline documents",
		slog.Int("markdown", len(discovered)),
		slog.Int("assets", len(assetFiles)))

	// Build repository metadata for generators
	repoMetadata := g.buildRepositoryMetadata()

	// Create and run pipeline processor
	processor := pipeline.NewProcessor(g.config)
	processedDocs, err := processor.ProcessContent(discovered, repoMetadata)
	if err != nil {
		return fmt.Errorf("%w: pipeline processing failed: %w",
			herrors.ErrContentTransformFailed, err)
	}

	slog.Info("Pipeline processing complete",
		slog.Int("input", len(discovered)),
		slog.Int("output", len(processedDocs)))

	// Write processed documents to Hugo content directory
	for i, doc := range processedDocs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Construct output path
		outputPath := filepath.Join(g.buildRoot(), doc.Path)

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
			return fmt.Errorf("%w: failed to create directory for %s: %w",
				herrors.ErrContentWriteFailed, outputPath, err)
		}

		// Write file
		// #nosec G306 -- content files are public documentation
		if err := os.WriteFile(outputPath, doc.Raw, 0o644); err != nil {
			return fmt.Errorf("%w: failed to write file %s: %w",
				herrors.ErrContentWriteFailed, outputPath, err)
		}

		slog.Debug("Wrote processed document",
			slog.String("path", doc.Path),
			slog.Int("bytes", len(doc.Raw)),
			slog.Bool("generated", doc.Generated))

		// Update page counter
		if g.onPageRendered != nil {
			g.onPageRendered()
		}

		// Store transformed bytes for index generation
		// Find corresponding DocFile and update it
		if i < len(markdownFiles) && !doc.Generated {
			markdownFiles[i].TransformedBytes = doc.Raw
		}
	}

	slog.Info("Copied all content files using pipeline",
		slog.Int("count", len(processedDocs)))

	return nil
}

// buildRepositoryMetadata extracts repository metadata for pipeline generators.
func (g *Generator) buildRepositoryMetadata() map[string]pipeline.RepositoryInfo {
	metadata := make(map[string]pipeline.RepositoryInfo)

	if g.config == nil || g.config.Repositories == nil {
		return metadata
	}

	for _, repo := range g.config.Repositories {
		info := pipeline.RepositoryInfo{
			Name:     repo.Name,
			URL:      repo.URL,
			Branch:   repo.Branch,
			Tags:     repo.Tags,
			DocsBase: "docs", // Default
		}

		// Get forge type from tags
		if forgeType, ok := repo.Tags["forge_type"]; ok {
			info.Forge = forgeType
		}

		// Get docs base from paths (use first path if multiple)
		if len(repo.Paths) > 0 {
			info.DocsBase = repo.Paths[0]
		}

		// TODO: Get commit SHA from git client
		// For now, leave empty - will be populated by metadata injector

		metadata[repo.Name] = info
	}

	return metadata
}
