package hugo

import (
	"log/slog"
	"os/exec"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// shouldRunHugo determines if we should invoke the external hugo binary.
// In RenderModeAuto it no longer consults legacy environment variables; the
// effective behavior is derived solely from configuration and CLI flags via
// ResolveEffectiveRenderMode.

func shouldRunHugo(cfg *config.Config) bool {
	mode := config.ResolveEffectiveRenderMode(cfg)
	switch mode {
	case config.RenderModeNever:
		return false
	case config.RenderModeAlways:
		if _, err := exec.LookPath("hugo"); err != nil {
			slog.Warn("Hugo binary not found while in render_mode=always; skipping execution", "error", err)
			return false
		}
		return true
	case config.RenderModeAuto:
		// Auto mode: defer to ResolveEffectiveRenderMode only. If the effective
		// mode is still auto at this point, treat it as "do not run".
		return false
	default:
		return false
	}
}
