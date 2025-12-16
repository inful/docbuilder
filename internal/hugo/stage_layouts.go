package hugo

import (
	"context"
)

func stageLayouts(_ context.Context, bs *BuildState) error {
	// Copy transition assets if enabled (before checking theme)
	if err := bs.Generator.copyTransitionAssets(); err != nil {
		return err
	}
	// Copy taxonomy layouts if taxonomies are configured
	if err := bs.Generator.copyTaxonomyLayouts(); err != nil {
		return err
	}
	if bs.Generator.config != nil && bs.Generator.config.Hugo.Theme != "" {
		var _ = bs.Generator.config.Hugo.Theme
		return nil
	}
	return bs.Generator.generateBasicLayouts()
}
