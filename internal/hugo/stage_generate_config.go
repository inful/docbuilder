package hugo

import (
	"context"
)

func stageGenerateConfig(_ context.Context, bs *BuildState) error {
	// Ensure ConfigHash derived from unified snapshot if not already populated (direct path sets earlier).
	if bs.Pipeline.ConfigHash == "" {
		bs.Pipeline.ConfigHash = bs.Generator.computeConfigHash()
		if bs.Report != nil {
			bs.Report.ConfigHash = bs.Pipeline.ConfigHash
		}
	}
	return bs.Generator.generateHugoConfig()
}
