package hugo

import (
	"context"
)

func stageGenerateConfig(ctx context.Context, bs *BuildState) error {
	// Ensure ConfigHash derived from unified snapshot if not already populated (direct path sets earlier).
	if bs.ConfigHash == "" {
		bs.ConfigHash = bs.Generator.computeConfigHash()
		if bs.Report != nil {
			bs.Report.ConfigHash = bs.ConfigHash
		}
	}
	return bs.Generator.generateHugoConfig()
}
