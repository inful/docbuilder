package hugo

import "context"

func stageLayouts(ctx context.Context, bs *BuildState) error {
	if bs.Generator.config != nil && bs.Generator.config.Hugo.Theme != "" {
		var _ = bs.Generator.config.Hugo.Theme
		return nil
	}
	return bs.Generator.generateBasicLayouts()
}
