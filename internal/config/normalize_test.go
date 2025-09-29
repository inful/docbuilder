package config

import "testing"

func TestNormalizeConfigEnums(t *testing.T) {
	cfg := &Config{Version: "2.0", Build: BuildConfig{
		RenderMode:      "AlWaYs", // case mixed
		NamespaceForges: "ALWAYS",
		CloneStrategy:   "AuTo",
		RetryBackoff:    "ExPoNeNtIaL",
		CloneConcurrency: -5,
		ShallowDepth:    -2,
	}}
	res, err := NormalizeConfig(cfg)
	if err != nil { t.Fatalf("NormalizeConfig error: %v", err) }
	if cfg.Build.RenderMode != RenderModeAlways { t.Fatalf("render_mode not normalized: %v", cfg.Build.RenderMode) }
	if cfg.Build.NamespaceForges != NamespacingAlways { t.Fatalf("namespace_forges not normalized: %v", cfg.Build.NamespaceForges) }
	if cfg.Build.CloneStrategy != CloneStrategyAuto { t.Fatalf("clone_strategy not normalized: %v", cfg.Build.CloneStrategy) }
	if cfg.Build.RetryBackoff != RetryBackoffExponential { t.Fatalf("retry_backoff not normalized: %v", cfg.Build.RetryBackoff) }
	if cfg.Build.CloneConcurrency != 0 { t.Fatalf("negative clone_concurrency not clamped: %d", cfg.Build.CloneConcurrency) }
	if cfg.Build.ShallowDepth != 0 { t.Fatalf("negative shallow_depth not clamped: %d", cfg.Build.ShallowDepth) }
	if len(res.Warnings) == 0 { t.Fatalf("expected warnings recorded") }
}

func TestNormalizeConfigUnknowns(t *testing.T) {
	cfg := &Config{Version: "2.0", Build: BuildConfig{
		RenderMode:      "gibberish",
		NamespaceForges: "???",
		CloneStrategy:   "mystery",
		RetryBackoff:    "spiral",
	}}
	res, err := NormalizeConfig(cfg)
	if err != nil { t.Fatalf("NormalizeConfig error: %v", err) }
	if cfg.Build.RenderMode != RenderModeAuto { t.Fatalf("render_mode fallback failed: %v", cfg.Build.RenderMode) }
	if cfg.Build.NamespaceForges != NamespacingAuto { t.Fatalf("namespace_forges fallback failed: %v", cfg.Build.NamespaceForges) }
	if cfg.Build.CloneStrategy != CloneStrategyFresh { t.Fatalf("clone_strategy fallback failed: %v", cfg.Build.CloneStrategy) }
	if cfg.Build.RetryBackoff != RetryBackoffFixed { t.Fatalf("retry_backoff fallback failed: %v", cfg.Build.RetryBackoff) }
	if len(res.Warnings) < 4 { t.Fatalf("expected >=4 warnings, got %d", len(res.Warnings)) }
}
