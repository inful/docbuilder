package stages

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// StagePostProcess does the last work on the staging directory. It has
// none to do: the stage is in the list of the stages, and the stages
// are reported by name.
func StagePostProcess(_ context.Context, _ *models.BuildState) error {
	return nil
}
