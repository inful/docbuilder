package hugo

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"

	"git.home.luguber.info/inful/docbuilder/internal/docs"

	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/pipeline"
)

// copyContentFilesPipeline copies documentation files using the new fixed transform pipeline.
// This is the new implementation that replaces the registry-based transform system.
func (g *Generator) copyContentFilesPipeline(ctx context.Context, docFiles []docs.DocFile, bs *models.BuildState) error {
	slog.Info("Using new fixed transform pipeline for content processing")
	publicOnly := g.config.IsDaemonPublicOnlyEnabled()

	// Compute isSingleRepo flag
	var isSingleRepo bool
	if bs != nil {
		isSingleRepo = bs.Docs.IsSingleRepo
	} else {
		// Fallback: compute from docFiles when models.BuildState is nil (e.g., in tests)
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
			return derrors.WrapError(err, derrors.CategoryFileSystem, "failed to copy asset").
				WithContext("path", file.Path).
				Build()
		}
	}

	// Convert DocFiles to pipeline Documents
	discovered := make([]*pipeline.Document, 0, len(markdownFiles))
	excluded := 0
	for i := range markdownFiles {
		file := &markdownFiles[i]
		// Load content
		if err := file.LoadContent(); err != nil {
			return derrors.WrapError(err, derrors.CategoryInternal, "failed to load content").
				WithCause(herrors.ErrContentTransformFailed).
				WithContext("path", file.Path).
				Build()
		}

		if publicOnly && !isPublicMarkdown(file.Content) {
			excluded++
			continue
		}

		// Convert to pipeline Document
		doc := pipeline.NewDocumentFromDocFile(*file, isSingleRepo, g.config.Build.IsPreview, g.config.Build.VSCodeEditLinks, g.config.Build.EditURLBase)
		discovered = append(discovered, doc)
	}

	slog.Info("Converted discovered files to pipeline documents",
		slog.Int("markdown", len(discovered)),
		slog.Int("assets", len(assetFiles)))
	if publicOnly {
		slog.Info("Daemon public-only filter applied",
			slog.Int("excluded_markdown", excluded),
			slog.Int("included_markdown", len(discovered)))
	}

	// Build repository metadata for generators
	repoMetadata := g.buildRepositoryMetadata(bs)

	// Create and run pipeline processor
	processor := pipeline.NewProcessor(g.config)
	processedDocs, err := processor.ProcessContent(discovered, repoMetadata, isSingleRepo)
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "pipeline processing failed").
			WithCause(herrors.ErrContentTransformFailed).
			Build()
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
		outputPath := filepath.Join(g.BuildRoot(), doc.Path)

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
			return derrors.WrapError(err, derrors.CategoryFileSystem, "failed to create directory").
				WithCause(herrors.ErrContentWriteFailed).
				WithContext("path", outputPath).
				Build()
		}

		contentBytes := doc.Raw

		// Write file
		// #nosec G306 -- content files are public documentation
		if err := os.WriteFile(outputPath, contentBytes, 0o644); err != nil {
			return derrors.WrapError(err, derrors.CategoryFileSystem, "failed to write file").
				WithCause(herrors.ErrContentWriteFailed).
				WithContext("path", outputPath).
				Build()
		}

		slog.Debug("Wrote processed document",
			slog.String("path", doc.Path),
			slog.Int("bytes", len(contentBytes)),
			slog.Bool("generated", doc.Generated))

		// Update page counter
		if g.onPageRendered != nil {
			g.onPageRendered()
		}

		// Store transformed bytes for index generation
		// Find corresponding DocFile and update it
		if i < len(markdownFiles) && !doc.Generated {
			markdownFiles[i].TransformedBytes = contentBytes
		}
	}

	slog.Info("Copied all content files using pipeline",
		slog.Int("count", len(processedDocs)))

	// Generate and write static assets (e.g., View Transitions)
	if err := g.generateStaticAssets(processor); err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "failed to generate static assets").Build()
	}

	return nil
}

// generateStaticAssets generates and writes static assets using pipeline generators.
func (g *Generator) generateStaticAssets(processor *pipeline.Processor) error {
	assets, err := processor.GenerateStaticAssets()
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "static asset generation failed").Build()
	}

	if len(assets) == 0 {
		return nil
	}

	slog.Info("Writing static assets", slog.Int("count", len(assets)))

	for _, asset := range assets {
		outputPath := filepath.Join(g.BuildRoot(), asset.Path)

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
			return derrors.WrapError(err, derrors.CategoryFileSystem, "failed to create directory").
				WithCause(herrors.ErrContentWriteFailed).
				WithContext("path", outputPath).
				Build()
		}

		// Write asset file
		// #nosec G306 -- static assets are public files
		if err := os.WriteFile(outputPath, asset.Content, 0o644); err != nil {
			return derrors.WrapError(err, derrors.CategoryFileSystem, "failed to write asset").
				WithCause(herrors.ErrContentWriteFailed).
				WithContext("path", outputPath).
				Build()
		}

		slog.Debug("Wrote static asset",
			slog.String("path", asset.Path),
			slog.Int("bytes", len(asset.Content)))
	}

	return nil
}

// buildRepositoryMetadata extracts repository metadata for pipeline generators.
// If bs is provided, uses commit dates from models.BuildState.
func (g *Generator) buildRepositoryMetadata(bs *models.BuildState) map[string]pipeline.RepositoryInfo {
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

		// Get commit SHA and date from models.BuildState if available
		if bs != nil {
			if commitSHA, ok := bs.Git.PostHeads[repo.Name]; ok {
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
	// Open the asset file
	src, err := os.Open(file.Path)
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryFileSystem, "failed to open asset").
			WithCause(herrors.ErrContentWriteFailed).
			WithContext("path", file.Path).
			Build()
	}
	defer func() {
		if cerr := src.Close(); cerr != nil {
			slog.Warn("Failed to close source file",
				slog.String("path", file.Path),
				slog.String("error", cerr.Error()))
		}
	}()

	// Check file size against limit
	info, err := src.Stat()
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryFileSystem, "failed to stat asset").
			WithCause(herrors.ErrContentWriteFailed).
			WithContext("path", file.Path).
			Build()
	}

	if g.config.Build.MaxAssetSize > 0 && info.Size() > g.config.Build.MaxAssetSize {
		slog.Warn("Skipping asset file: exceeds maximum allowed size",
			slog.String("path", file.Path),
			slog.Int64("size", info.Size()),
			slog.Int64("limit", g.config.Build.MaxAssetSize))
		return nil // Soft skip
	}

	// Calculate output path - assets go in same location as markdown files
	outputPath := filepath.Join(g.BuildRoot(), file.GetHugoPath(isSingleRepo))

	// Create directory if needed
	if mkdirErr := os.MkdirAll(filepath.Dir(outputPath), 0o750); mkdirErr != nil {
		return derrors.WrapError(mkdirErr, derrors.CategoryFileSystem, "failed to create directory").
			WithCause(herrors.ErrContentWriteFailed).
			WithContext("path", outputPath).
			Build()
	}

	// Stream the asset file as-is
	// #nosec G304 -- outputPath is constructed from trusted buildRoot and validated GetHugoPath
	dst, err := os.Create(outputPath)
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryFileSystem, "failed to create asset destination").
			WithCause(herrors.ErrContentWriteFailed).
			WithContext("path", outputPath).
			Build()
	}
	defer func() {
		if cerr := dst.Close(); cerr != nil {
			slog.Warn("Failed to close destination file",
				slog.String("path", outputPath),
				slog.String("error", cerr.Error()))
		}
	}()

	if _, err = io.Copy(dst, src); err != nil {
		return derrors.WrapError(err, derrors.CategoryFileSystem, "failed to copy asset").
			WithCause(herrors.ErrContentWriteFailed).
			WithContext("source", file.Path).
			WithContext("destination", outputPath).
			Build()
	}

	slog.Debug("Copied asset file",
		slog.String("source", file.RelativePath),
		slog.String("destination", file.GetHugoPath(isSingleRepo)),
		slog.String("type", file.Extension))

	return nil
}
