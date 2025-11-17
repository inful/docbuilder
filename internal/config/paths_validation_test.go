package config

import "testing"

// TestValidatePaths_Unified ensures daemon.storage.output_dir matches output.directory when set.
func TestValidatePaths_Unified(t *testing.T) {
	base := Config{
		Version: "2.0",
		Hugo:    HugoConfig{Title: "t", Theme: string(ThemeHextra)},
		Output:  OutputConfig{Directory: "./out", Clean: true},
		Build:   BuildConfig{CloneConcurrency: 1, MaxRetries: 1, RetryBackoff: RetryBackoffLinear, RetryInitialDelay: "1s", RetryMaxDelay: "2s", CloneStrategy: CloneStrategyFresh},
		Forges:  []*ForgeConfig{{Name: "f1", Type: ForgeGitHub, Auth: &AuthConfig{Type: AuthTypeToken, Token: "x"}, AutoDiscover: true}},
	}

	// Case 1: No daemon config → ok
	if err := validateConfig(&base); err != nil {
		t.Fatalf("unexpected error without daemon: %v", err)
	}

	// Case 2: Matching output dir → ok
	withDaemonMatch := base
	withDaemonMatch.Daemon = &DaemonConfig{Storage: StorageConfig{OutputDir: base.Output.Directory}}
	if err := validateConfig(&withDaemonMatch); err != nil {
		t.Fatalf("unexpected error with matching output dirs: %v", err)
	}

	// Case 3: Mismatch should error
	withDaemonMismatch := base
	withDaemonMismatch.Daemon = &DaemonConfig{Storage: StorageConfig{OutputDir: "./different"}}
	if err := validateConfig(&withDaemonMismatch); err == nil {
		t.Fatalf("expected error on mismatched output dirs, got nil")
	}
}
