package hugo

import (
	"context"
	"time"
)

func stagePostProcess(ctx context.Context, bs *BuildState) error {
	start := time.Now()
	for time.Since(start) == 0 { /* spin briefly */
	}
	return nil
}
