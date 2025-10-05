package version

import "testing"

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
	
	// Should have a default development version
	if Version == "unknown" {
		t.Error("Version should have a meaningful default value")
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