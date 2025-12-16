package hugo

import (
	"context"
)

func stageLayouts(_ context.Context, bs *BuildState) error {
	// Relearn theme provides all necessary layouts via Hugo Modules
	// No custom layout generation needed
	return nil
}
