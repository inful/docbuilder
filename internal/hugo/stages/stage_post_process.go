package stages

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

func StagePostProcess(_ context.Context, _ *models.BuildState) error {
	start := time.Now()
	// Brief spin to ensure distinguishable timestamps for build stages
	for time.Since(start) == 0 {
	}
	return nil
}
