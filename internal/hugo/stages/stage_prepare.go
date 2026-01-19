package stages

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// StagePrepareOutput creates the Hugo structure.
func StagePrepareOutput(_ context.Context, bs *models.BuildState) error {
	return bs.Generator.CreateHugoStructure()
}
