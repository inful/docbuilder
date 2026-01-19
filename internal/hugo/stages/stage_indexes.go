package stages

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// StageIndexes is now a no-op since the new pipeline (ADR-003) generates all
// indexes during content processing. Kept as empty function to maintain build
// stage compatibility.
func StageIndexes(_ context.Context, bs *models.BuildState) error {
	// New pipeline already generates all indexes - nothing to do here
	return nil
}
