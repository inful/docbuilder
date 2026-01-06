package hugo

import "context"

// stagePrepareOutput creates the Hugo structure and copies taxonomy layouts.
func stagePrepareOutput(_ context.Context, bs *BuildState) error {
	if err := bs.Generator.createHugoStructure(); err != nil {
		return err
	}
	// Copy custom taxonomy layouts for Relearn theme
	return bs.Generator.copyTaxonomyLayouts()
}
