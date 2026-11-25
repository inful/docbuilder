package build

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The build must not change the configuration of the caller: the hash
// of the configuration holds the clone strategy, and a flag that does
// not change the build must not look like a change of the configuration.
// The daemon keeps the configuration it was given, and the next build
// saw a different one.
func TestTheIncrementalBuildDoesNotChangeTheConfigurationOfTheCaller(t *testing.T) {
	svc := NewBuildService()

	cfg := &config.Config{
		Build:        config.BuildConfig{CloneStrategy: config.CloneStrategyFresh},
		Repositories: []config.Repository{{Name: "test", URL: "https://example.invalid/test.git"}},
	}
	before := cfg.Snapshot()

	// The build fails on the clone, and that does not matter here.
	_, _ = svc.Run(t.Context(), BuildRequest{
		Config:      cfg,
		OutputDir:   t.TempDir(),
		Incremental: true,
	})

	if cfg.Build.CloneStrategy != config.CloneStrategyFresh {
		t.Fatalf("the configuration of the caller was changed: %q", cfg.Build.CloneStrategy)
	}
	if after := cfg.Snapshot(); after != before {
		t.Fatalf("the hash of the configuration changed: %s -> %s", before, after)
	}
}
