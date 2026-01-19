package hugo

import "context"

// stagePrepareOutput creates the Hugo structure.
func stagePrepareOutput(_ context.Context, bs *BuildState) error {
	return bs.Generator.createHugoStructure()
}
