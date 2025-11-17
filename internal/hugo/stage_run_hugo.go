package hugo

import (
	"context"
	"fmt"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
)

func stageRunHugo(_ context.Context, bs *BuildState) error {
	cfg := bs.Generator.Config()
	mode := config.ResolveEffectiveRenderMode(cfg)
	if mode == config.RenderModeNever {
		return nil
	}

	// Use renderer abstraction; if custom renderer is set, use it regardless of shouldRunHugo
	root := bs.Generator.buildRoot()
	if bs.Generator.renderer != nil {
		if err := bs.Generator.renderer.Execute(root); err != nil {
			slog.Warn("Renderer execution failed", "error", err)
			return newWarnStageError(StageRunHugo, fmt.Errorf("%w: %v", herrors.ErrHugoExecutionFailed, err))
		}
		bs.Report.StaticRendered = true
		return nil
	}

	// No custom renderer; check if we should run the default hugo binary
	if !shouldRunHugo(cfg) {
		return nil
	}

	// Fallback to legacy method (should not happen in normal use, defensive)
	if err := bs.Generator.runHugoBuild(); err != nil {
		return newWarnStageError(StageRunHugo, fmt.Errorf("%w: %v", build.ErrHugo, err))
	}
	bs.Report.StaticRendered = true
	return nil
}
