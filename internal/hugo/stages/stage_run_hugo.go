package stages

import (
	"context"
	"fmt"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
)

func StageRunHugo(ctx context.Context, bs *models.BuildState) error {
	cfg := bs.Generator.Config()
	mode := config.ResolveEffectiveRenderMode(cfg)
	if mode == config.RenderModeNever {
		return nil
	}

	// Check if we should run Hugo based on mode and config
	if !shouldRunHugo(cfg) {
		// No rendering needed (e.g., auto mode without explicit request)
		// However, if a custom renderer is set (like NoopRenderer), we should still proceed
		if bs.Generator.Renderer() == nil {
			return nil
		}
		// Custom renderer is set, so we'll use it even if shouldRunHugo says no
	}

	// Use renderer abstraction; if custom renderer is set, use it, otherwise use default BinaryRenderer
	root := bs.Generator.BuildRoot()
	renderer := bs.Generator.Renderer()
	if renderer == nil {
		renderer = &BinaryRenderer{}
	}
	slog.Info("Executing Hugo renderer",
		slog.String("root", root),
		slog.String("renderer_type", fmt.Sprintf("%T", renderer)))
	if err := renderer.Execute(ctx, root); err != nil {
		slog.Error("Renderer execution failed",
			slog.String("error", err.Error()),
			slog.String("root", root))
		// Return error regardless of mode - let caller decide how to handle
		return models.NewFatalStageError(models.StageRunHugo, fmt.Errorf("%w: %w", herrors.ErrHugoExecutionFailed, err))
	}
	bs.Report.StaticRendered = true
	slog.Info("Hugo renderer completed successfully",
		slog.String("root", root),
		slog.Bool("static_rendered", true))
	return nil
}
