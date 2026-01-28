package config

import "testing"

func TestDaemonBuildDebounce_DefaultsAreValid(t *testing.T) {
	cfg := &DaemonConfig{}
	applyDaemonBuildDebounceDefaults(cfg)

	if cfg.BuildDebounce == nil {
		t.Fatalf("expected BuildDebounce defaults to create non-nil config")
	}

	if err := validateDaemonBuildDebounce(cfg.BuildDebounce); err != nil {
		t.Fatalf("daemon build debounce defaults violate validation rules: %v", err)
	}
}
