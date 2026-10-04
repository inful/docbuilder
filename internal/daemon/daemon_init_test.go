package daemon

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestGetBuildDebounceDurations_Defaults locks the default fallback
// when the daemon config has no BuildDebounce block at all. The
// defaults are part of the daemon's operational contract; changing
// them changes how aggressively it coalesces build requests.
func TestGetBuildDebounceDurations_Defaults(t *testing.T) {
	quiet, maxDelay, err := getBuildDebounceDurations(&config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quiet != 10*1_000_000_000 { // 10s
		t.Errorf("quiet = %v, want 10s", quiet)
	}
	if maxDelay != 60*1_000_000_000 { // 60s
		t.Errorf("max = %v, want 60s", maxDelay)
	}
}

// TestGetBuildDebounceDurations_NilDaemonSafe verifies the helper
// never panics on the partial configs the constructor builds before
// fully populating the DaemonConfig.
func TestGetBuildDebounceDurations_NilDaemonSafe(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
	}{
		{"nil config", nil},
		{"nil daemon block", &config.Config{}},
		{"nil build_debounce block", &config.Config{Daemon: &config.DaemonConfig{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := getBuildDebounceDurations(tc.cfg); err != nil {
				t.Errorf("unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}

// TestGetBuildDebounceDurations_InvalidReturnsError locks the error
// contract: a malformed quiet_window or max_delay string must be
// surfaced by the helper so the constructor can return a clear error.
// This is the regression guard for the initBuildDebouncer step in
// NewDaemonWithConfigFile.
//
// Whitespace-only values are NOT errors: the helper trims and falls
// back to the default. That is intentional — see the comment in
// getBuildDebounceDurations and the dedicated coverage in
// TestGetBuildDebounceDurations_WhitespaceFallsBackToDefault.
func TestGetBuildDebounceDurations_InvalidReturnsError(t *testing.T) {
	cases := []struct {
		name        string
		quietWindow string
		maxDelay    string
	}{
		{"invalid quiet_window", "not-a-duration", "60s"},
		{"invalid max_delay", "10s", "garbage"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Daemon: &config.DaemonConfig{
					BuildDebounce: &config.BuildDebounceConfig{
						QuietWindow: tc.quietWindow,
						MaxDelay:    tc.maxDelay,
					},
				},
			}
			_, _, err := getBuildDebounceDurations(cfg)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			// Error must mention which field failed; operators reading
			// the message should not have to guess.
			if !strings.Contains(err.Error(), "build_debounce") {
				t.Errorf("error %q must mention build_debounce to identify the source", err.Error())
			}
		})
	}
}

// TestGetBuildDebounceDurations_WhitespaceFallsBackToDefault locks the
// lenient-fallback contract: a whitespace-only value is treated as
// "use the default" rather than "error out". This is the behavior
// operators get when their config has the field present but empty.
func TestGetBuildDebounceDurations_WhitespaceFallsBackToDefault(t *testing.T) {
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			BuildDebounce: &config.BuildDebounceConfig{
				QuietWindow: "  ",
				MaxDelay:    "  ",
			},
		},
	}
	quiet, maxDelay, err := getBuildDebounceDurations(cfg)
	if err != nil {
		t.Fatalf("whitespace should fall back to default, got error: %v", err)
	}
	if quiet != 10*1_000_000_000 {
		t.Errorf("quiet = %v, want 10s (default)", quiet)
	}
	if maxDelay != 60*1_000_000_000 {
		t.Errorf("max = %v, want 60s (default)", maxDelay)
	}
}

// TestGetBuildDebounceDurations_OverridesApplied verifies the
// helper honors non-empty explicit values for either field while
// leaving the other at its default.
func TestGetBuildDebounceDurations_OverridesApplied(t *testing.T) {
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			BuildDebounce: &config.BuildDebounceConfig{
				QuietWindow: "5s",
				MaxDelay:    "30s",
			},
		},
	}
	quiet, maxDelay, err := getBuildDebounceDurations(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quiet != 5*1_000_000_000 {
		t.Errorf("quiet = %v, want 5s", quiet)
	}
	if maxDelay != 30*1_000_000_000 {
		t.Errorf("max = %v, want 30s", maxDelay)
	}
}
