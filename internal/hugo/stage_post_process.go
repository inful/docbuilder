package hugo

import (
	"context"
	"time"
)

func stagePostProcess(_ context.Context, _ *BuildState) error {
	start := time.Now()
	// Brief spin to ensure distinguishable timestamps for build stages
	for time.Since(start) == 0 {
	}
	return nil
}
