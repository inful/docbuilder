package config

import "testing"

func TestValidateDaemonSyncSchedule_ValidCron(t *testing.T) {
	base := Config{
		Version: "2.0",
		Output:  OutputConfig{Directory: "./out", Clean: true},
		Build:   BuildConfig{CloneConcurrency: 1, MaxRetries: 1, RetryBackoff: RetryBackoffLinear, RetryInitialDelay: "1s", RetryMaxDelay: "2s", CloneStrategy: CloneStrategyFresh},
		Forges:  []*ForgeConfig{{Name: "f1", Type: ForgeGitHub, Auth: &AuthConfig{Type: AuthTypeToken, Token: "x"}, AutoDiscover: true}},
		Daemon: &DaemonConfig{
			Sync: SyncConfig{Schedule: "0 */4 * * *"},
		},
	}

	if err := validateConfig(&base); err != nil {
		t.Fatalf("unexpected error for valid cron schedule: %v", err)
	}
}

func TestValidateDaemonSyncSchedule_InvalidCron(t *testing.T) {
	base := Config{
		Version: "2.0",
		Output:  OutputConfig{Directory: "./out", Clean: true},
		Build:   BuildConfig{CloneConcurrency: 1, MaxRetries: 1, RetryBackoff: RetryBackoffLinear, RetryInitialDelay: "1s", RetryMaxDelay: "2s", CloneStrategy: CloneStrategyFresh},
		Forges:  []*ForgeConfig{{Name: "f1", Type: ForgeGitHub, Auth: &AuthConfig{Type: AuthTypeToken, Token: "x"}, AutoDiscover: true}},
		Daemon: &DaemonConfig{
			Sync: SyncConfig{Schedule: "this is not a cron"},
		},
	}

	if err := validateConfig(&base); err == nil {
		t.Fatalf("expected error for invalid cron schedule, got nil")
	}
}

func TestValidateDaemonSyncSchedule_EmptyAfterTrim(t *testing.T) {
	base := Config{
		Version: "2.0",
		Output:  OutputConfig{Directory: "./out", Clean: true},
		Build:   BuildConfig{CloneConcurrency: 1, MaxRetries: 1, RetryBackoff: RetryBackoffLinear, RetryInitialDelay: "1s", RetryMaxDelay: "2s", CloneStrategy: CloneStrategyFresh},
		Forges:  []*ForgeConfig{{Name: "f1", Type: ForgeGitHub, Auth: &AuthConfig{Type: AuthTypeToken, Token: "x"}, AutoDiscover: true}},
		Daemon: &DaemonConfig{
			Sync: SyncConfig{Schedule: "   \t  "},
		},
	}

	if err := validateConfig(&base); err == nil {
		t.Fatalf("expected error for empty cron schedule, got nil")
	}
}
