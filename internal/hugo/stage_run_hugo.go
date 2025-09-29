package hugo

import (
	"context"
	"fmt"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
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
	// Use renderer abstraction; fallback to legacy method if unset (should not happen, defensive)
	root := bs.Generator.buildRoot()
	if bs.Generator.renderer == nil {
		if err := bs.Generator.runHugoBuild(); err != nil {
			return newWarnStageError(StageRunHugo, fmt.Errorf("%w: %v", build.ErrHugo, err))
		}
		bs.Report.StaticRendered = true
		return nil
	}
	if err := bs.Generator.renderer.Execute(root); err != nil {
		slog.Warn("Renderer execution failed", "error", err)
		return newWarnStageError(StageRunHugo, fmt.Errorf("%w: %v", herrors.ErrHugoExecutionFailed, err))
	}
	bs.Report.StaticRendered = true
	return nil
}
