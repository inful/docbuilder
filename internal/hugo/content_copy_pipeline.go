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
func (g *Generator) copyContentFilesPipeline(ctx context.Context, docFiles []docs.DocFile, bs *BuildState) error {
	slog.Info("Using new fixed transform pipeline for content processing")

	// Compute isSingleRepo flag
	var isSingleRepo bool
	if bs != nil {
		isSingleRepo = bs.Docs.IsSingleRepo
	} else {
		// Fallback: compute from docFiles when BuildState is nil (e.g., in tests)
		repoSet := make(map[string]struct{})
		for i := range docFiles {
			repoSet[docFiles[i].Repository] = struct{}{}
		}
		isSingleRepo = len(repoSet) == 1
	}

	// Separate markdown files from assets
	var markdownFiles []docs.DocFile
	var assetFiles []docs.DocFile

	for i := range docFiles {
		file := &docFiles[i]
		if file.IsAsset {
			assetFiles = append(assetFiles, *file)
		} else {
			markdownFiles = append(markdownFiles, *file)
		}
	}

	// Process assets first (simple copy)
	for i := range assetFiles {
		file := &assetFiles[i]
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := g.copyAssetFile(*file, isSingleRepo); err != nil {
			return fmt.Errorf("failed to copy asset %s: %w", file.Path, err)
		}
	}

	// Convert DocFiles to pipeline Documents
	discovered := make([]*pipeline.Document, 0, len(markdownFiles))
	for i := range markdownFiles {
		file := &markdownFiles[i]
		// Load content
		if err := file.LoadContent(); err != nil {
			return fmt.Errorf("%w: failed to load content for %s: %w",
				herrors.ErrContentTransformFailed, file.Path, err)
		}

		// Convert to pipeline Document
		doc := pipeline.NewDocumentFromDocFile(*file, isSingleRepo, g.config.Build.IsPreview, g.config.Build.EditURLBase)
		discovered = append(discovered, doc)
	}

	slog.Info("Converted discovered files to pipeline documents",
		slog.Int("markdown", len(discovered)),
		slog.Int("assets", len(assetFiles)))

	// Build repository metadata for generators
	repoMetadata := g.buildRepositoryMetadata(bs)

	// Create and run pipeline processor
	processor := pipeline.NewProcessor(g.config)
	processedDocs, err := processor.ProcessContent(discovered, repoMetadata, isSingleRepo)
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

	// Generate and write static assets (e.g., View Transitions)
	if err := g.generateStaticAssets(processor); err != nil {
		return fmt.Errorf("failed to generate static assets: %w", err)
	}

	return nil
}

// generateStaticAssets generates and writes static assets using pipeline generators.
func (g *Generator) generateStaticAssets(processor *pipeline.Processor) error {
	assets, err := processor.GenerateStaticAssets()
	if err != nil {
		return fmt.Errorf("static asset generation failed: %w", err)
	}

	if len(assets) == 0 {
		return nil
	}

	slog.Info("Writing static assets", slog.Int("count", len(assets)))

	for _, asset := range assets {
		outputPath := filepath.Join(g.buildRoot(), asset.Path)

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
			return fmt.Errorf("%w: failed to create directory for %s: %w",
				herrors.ErrContentWriteFailed, outputPath, err)
		}

		// Write asset file
		// #nosec G306 -- static assets are public files
		if err := os.WriteFile(outputPath, asset.Content, 0o644); err != nil {
			return fmt.Errorf("%w: failed to write asset %s: %w",
				herrors.ErrContentWriteFailed, outputPath, err)
		}

		slog.Debug("Wrote static asset",
			slog.String("path", asset.Path),
			slog.Int("bytes", len(asset.Content)))
	}

	return nil
}

// buildRepositoryMetadata extracts repository metadata for pipeline generators.
// If bs is provided, uses commit dates from BuildState.
func (g *Generator) buildRepositoryMetadata(bs *BuildState) map[string]pipeline.RepositoryInfo {
	metadata := make(map[string]pipeline.RepositoryInfo)

	if g.config == nil || g.config.Repositories == nil {
		return metadata
	}

	for i := range g.config.Repositories {
		repo := &g.config.Repositories[i]
		info := pipeline.RepositoryInfo{
			Name:      repo.Name,
			URL:       repo.URL,
			Branch:    repo.Branch,
			Tags:      repo.Tags,
			DocsBase:  "docs", // Default
			DocsPaths: []string{"docs"},
		}

		// Get forge type from tags
		if forgeType, ok := repo.Tags["forge_type"]; ok {
			info.Forge = forgeType
		}

		// Get docs base from paths (use first path if multiple)
		if len(repo.Paths) > 0 {
			info.DocsBase = repo.Paths[0]
			info.DocsPaths = repo.Paths
		}

		// Get commit SHA and date from BuildState if available
		if bs != nil {
			if commitSHA, ok := bs.Git.postHeads[repo.Name]; ok {
				info.Commit = commitSHA
			}
			if commitDate, ok := bs.Git.GetCommitDate(repo.Name); ok {
				info.CommitDate = commitDate
			}
		}

		metadata[repo.Name] = info
	}

	return metadata
}

// copyAssetFile copies an asset file (image, etc.) to Hugo content directory without processing.
func (g *Generator) copyAssetFile(file docs.DocFile, isSingleRepo bool) error {
	// Read the asset file
	content, err := os.ReadFile(file.Path)
	if err != nil {
		return fmt.Errorf("%w: failed to read asset %s: %w",
			herrors.ErrContentWriteFailed, file.Path, err)
	}

	// Calculate output path - assets go in same location as markdown files
	outputPath := filepath.Join(g.buildRoot(), file.GetHugoPath(isSingleRepo))

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return fmt.Errorf("%w: failed to create directory for %s: %w",
			herrors.ErrContentWriteFailed, outputPath, err)
	}

	// Copy asset file as-is
	// #nosec G306 -- asset files are public documentation resources
	if err := os.WriteFile(outputPath, content, 0o644); err != nil {
		return fmt.Errorf("%w: failed to write asset %s: %w",
			herrors.ErrContentWriteFailed, outputPath, err)
	}

	slog.Debug("Copied asset file",
		slog.String("source", file.RelativePath),
		slog.String("destination", file.GetHugoPath(isSingleRepo)),
		slog.String("type", file.Extension))

	return nil
}
