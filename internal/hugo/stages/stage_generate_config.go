package stages

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

func StageGenerateConfig(_ context.Context, bs *models.BuildState) error {
	// Ensure ConfigHash derived from unified snapshot if not already populated (direct path sets earlier).
	if bs.Pipeline.ConfigHash == "" {
		bs.Pipeline.ConfigHash = bs.Generator.ComputeConfigHash()
		if bs.Report != nil {
			bs.Report.ConfigHash = bs.Pipeline.ConfigHash
		}
	}
	return bs.Generator.GenerateHugoConfig()
}
