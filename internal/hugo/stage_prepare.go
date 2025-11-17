package hugo

import "context"

// stagePrepareOutput currently creates the Hugo structure (no-op beyond ensuring dirs).
func stagePrepareOutput(_ context.Context, bs *BuildState) error {
	return bs.Generator.createHugoStructure()
}
