package config

import (
	"log/slog"
	"os"
)

// ResolveEffectiveRenderMode determines final render decision honoring new config field
// while maintaining backward compatibility with legacy environment variables.
// Precedence:
// 1. DOCBUILDER_SKIP_HUGO=1 => never
// 2. DOCBUILDER_RUN_HUGO=1 => always
// 3. build.render_mode (always|never|auto)
// 4. fallback: auto
func ResolveEffectiveRenderMode(cfg *Config) RenderMode {
	if os.Getenv("DOCBUILDER_SKIP_HUGO") == "1" {
		if cfg != nil && cfg.Build.RenderMode != RenderModeNever {
			slog.Info("Overriding configured render_mode due to DOCBUILDER_SKIP_HUGO=1", "configured", cfg.Build.RenderMode)
		}
		return RenderModeNever
	}
	if os.Getenv("DOCBUILDER_RUN_HUGO") == "1" {
		if cfg != nil && cfg.Build.RenderMode != RenderModeAlways {
			slog.Info("Overriding configured render_mode due to DOCBUILDER_RUN_HUGO=1", "configured", cfg.Build.RenderMode)
		}
		return RenderModeAlways
	}
	if cfg == nil {
		return RenderModeAuto
	}
	switch cfg.Build.RenderMode {
	case RenderModeAlways:
		return RenderModeAlways
	case RenderModeNever:
		return RenderModeNever
	default:
		return RenderModeAuto
	}
}
