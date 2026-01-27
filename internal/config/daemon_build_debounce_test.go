package config

import "testing"

func TestDaemonBuildDebounceDefaultsApplied(t *testing.T) {
	cfg := Config{Daemon: &DaemonConfig{}}
	if err := applyDefaults(&cfg); err != nil {
		t.Fatalf("defaults: %v", err)
	}

	if cfg.Daemon.BuildDebounce == nil {
		t.Fatalf("expected daemon.build_debounce to be defaulted")
	}
	if cfg.Daemon.BuildDebounce.QuietWindow != defaultDuration10s {
		t.Fatalf("expected quiet_window default 10s, got %q", cfg.Daemon.BuildDebounce.QuietWindow)
	}
	if cfg.Daemon.BuildDebounce.MaxDelay != defaultDuration60s {
		t.Fatalf("expected max_delay default 60s, got %q", cfg.Daemon.BuildDebounce.MaxDelay)
	}
	if cfg.Daemon.BuildDebounce.WebhookImmediate == nil {
		t.Fatalf("expected webhook_immediate default true")
	}
	if !*cfg.Daemon.BuildDebounce.WebhookImmediate {
		t.Fatalf("expected webhook_immediate default true")
	}
}

func TestValidateConfig_DaemonBuildDebounce_InvalidQuietWindow(t *testing.T) {
	cfg := Config{
		Version:      "2.0",
		Repositories: []Repository{{Name: "r"}},
		Daemon: &DaemonConfig{
			Sync: SyncConfig{Schedule: "0 */4 * * *"},
			BuildDebounce: &BuildDebounceConfig{
				QuietWindow: "nope",
				MaxDelay:    "60s",
			},
		},
	}
	if err := applyDefaults(&cfg); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	cfg.Daemon.BuildDebounce.QuietWindow = "nope"

	if err := ValidateConfig(&cfg); err == nil {
		t.Fatalf("expected validation error for invalid quiet_window")
	}
}

func TestValidateConfig_DaemonBuildDebounce_MaxDelayLessThanQuietWindow(t *testing.T) {
	cfg := Config{
		Version:      "2.0",
		Repositories: []Repository{{Name: "r"}},
		Daemon: &DaemonConfig{
			Sync: SyncConfig{Schedule: "0 */4 * * *"},
			BuildDebounce: &BuildDebounceConfig{
				QuietWindow: "10s",
				MaxDelay:    "5s",
			},
		},
	}
	if err := applyDefaults(&cfg); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	cfg.Daemon.BuildDebounce.QuietWindow = "10s"
	cfg.Daemon.BuildDebounce.MaxDelay = "5s"

	if err := ValidateConfig(&cfg); err == nil {
		t.Fatalf("expected validation error for max_delay < quiet_window")
	}
}
