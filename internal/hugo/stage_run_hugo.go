package hugo

import (
	"context"
	"fmt"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
)

func stageRunHugo(_ context.Context, bs *BuildState) error {
	cfg := bs.Generator.Config()
	mode := config.ResolveEffectiveRenderMode(cfg)
	if mode == config.RenderModeNever {
		return nil
	}

	// Check if we should run Hugo based on mode and config
	if !shouldRunHugo(cfg) {
		// No rendering needed (e.g., auto mode without explicit request)
		return nil
	}

	// Use renderer abstraction; if custom renderer is set, use it, otherwise use default BinaryRenderer
	root := bs.Generator.buildRoot()
	renderer := bs.Generator.renderer
	if renderer == nil {
		renderer = &BinaryRenderer{}
	}
	if err := renderer.Execute(root); err != nil {
		// In live preview mode, surface as warning only and continue serving prior build
		if cfg.Build.LiveReload {
			slog.Warn("Renderer execution failed (preview mode)", "error", err)
			return nil
		}
		slog.Warn("Renderer execution failed", "error", err)
		return newFatalStageError(StageRunHugo, fmt.Errorf("%w: %v", herrors.ErrHugoExecutionFailed, err))
	}
	bs.Report.StaticRendered = true
	return nil
}
