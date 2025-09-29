package hugo

import (
	"context"
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func stageRunHugo(ctx context.Context, bs *BuildState) error {
	cfg := bs.Generator.Config()
	mode := config.ResolveEffectiveRenderMode(cfg)
	if mode == config.RenderModeNever {
		return nil
	}
	if !shouldRunHugo(cfg) {
		return nil
	}
	if err := bs.Generator.runHugoBuild(); err != nil {
		return newWarnStageError(StageRunHugo, fmt.Errorf("%w: %v", build.ErrHugo, err))
	}
	bs.Report.StaticRendered = true
	return nil
}
