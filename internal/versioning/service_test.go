package versioning

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestGetVersioningConfig_Defaults(t *testing.T) {
	// Test that when v2Config.Versioning is nil, we get expected defaults
	v2Config := &config.Config{
		Versioning: nil,
	}

	result := GetVersioningConfig(v2Config)

	if result.Strategy != StrategyDefaultOnly {
		t.Errorf("expected Strategy to be StrategyDefaultOnly, got %v", result.Strategy)
	}

	if !result.DefaultBranchOnly {
		t.Error("expected DefaultBranchOnly to be true")
	}

	expectedBranchPatterns := []string{"main", "master"}
	if len(result.BranchPatterns) != len(expectedBranchPatterns) {
		t.Errorf("expected %d branch patterns, got %d", len(expectedBranchPatterns), len(result.BranchPatterns))
	}

	if len(result.TagPatterns) != 0 {
		t.Errorf("expected 0 tag patterns, got %d", len(result.TagPatterns))
	}

	expectedMaxVersions := 5
	if result.MaxVersions != expectedMaxVersions {
		t.Errorf("expected MaxVersions to be %d, got %d", expectedMaxVersions, result.MaxVersions)
	}
}

func TestGetVersioningConfig_WithCustomConfig(t *testing.T) {
	// Test that custom config is properly mapped
	v2Config := &config.Config{
		Versioning: &config.VersioningConfig{
			Strategy:            config.StrategyTagsOnly,
			DefaultBranchOnly:   false,
			BranchPatterns:      []string{"develop"},
			TagPatterns:         []string{"v*"},
			MaxVersionsPerRepo:  10,
		},
	}

	result := GetVersioningConfig(v2Config)

	if result.Strategy != StrategyTags {
		t.Errorf("expected Strategy to be StrategyTags, got %v", result.Strategy)
	}

	if result.DefaultBranchOnly {
		t.Error("expected DefaultBranchOnly to be false")
	}

	if len(result.BranchPatterns) != 1 || result.BranchPatterns[0] != "develop" {
		t.Errorf("expected branch patterns [develop], got %v", result.BranchPatterns)
	}

	if len(result.TagPatterns) != 1 || result.TagPatterns[0] != "v*" {
		t.Errorf("expected tag patterns [v*], got %v", result.TagPatterns)
	}

	if result.MaxVersions != 10 {
		t.Errorf("expected MaxVersions to be 10, got %d", result.MaxVersions)
	}
}
