package hugo

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// copyContentFiles copies documentation files to Hugo content directory.
// Uses the fixed transform pipeline (ADR-003).
func (g *Generator) copyContentFiles(ctx context.Context, docFiles []docs.DocFile) error {
	return g.copyContentFilesPipeline(ctx, docFiles, nil)
}

// copyContentFilesWithState copies documentation files with access to BuildState for metadata.
func (g *Generator) copyContentFilesWithState(ctx context.Context, docFiles []docs.DocFile, bs *BuildState) error {
	return g.copyContentFilesPipeline(ctx, docFiles, bs)
}
