package config

// ResolveEffectiveRenderMode determines final render decision based on the
// configured render_mode and any CLI overrides.
func ResolveEffectiveRenderMode(cfg *Config) RenderMode {
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
