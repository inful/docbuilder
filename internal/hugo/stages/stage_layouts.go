package stages

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

func StageLayouts(_ context.Context, bs *models.BuildState) error {
	// Relearn theme provides all necessary layouts via Hugo Modules
	// No custom layout generation needed
	return nil
}
