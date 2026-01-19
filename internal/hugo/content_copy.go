package hugo

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// copyContentFiles copies documentation files to Hugo content directory.
// Uses the fixed transform pipeline (ADR-003).
func (g *Generator) copyContentFiles(ctx context.Context, docFiles []docs.DocFile) error {
	return g.copyContentFilesPipeline(ctx, docFiles, nil)
}

// CopyContentFilesWithState copies documentation files with access to models.BuildState for metadata.
func (g *Generator) CopyContentFilesWithState(ctx context.Context, docFiles []docs.DocFile, bs *models.BuildState) error {
	return g.copyContentFilesPipeline(ctx, docFiles, bs)
}
