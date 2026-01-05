package version

import "testing"

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}

	// Default value should be "unknown" until set by build
	if Version != "unknown" {
		// In tests, version should be "unknown" unless explicitly set via ldflags
		t.Logf("Version is: %s (expected 'unknown' or version set via ldflags)", Version)
	}
}

func TestBuildInfo(t *testing.T) {
	// Build info variables should exist (even if set to "unknown")
	if BuildTime == "" {
		t.Error("BuildTime should be initialized")
	}

	if GitCommit == "" {
		t.Error("GitCommit should be initialized")
	}
}
