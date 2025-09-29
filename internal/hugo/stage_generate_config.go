package hugo

import "context"

func stageGenerateConfig(ctx context.Context, bs *BuildState) error {
	return bs.Generator.generateHugoConfig()
}
