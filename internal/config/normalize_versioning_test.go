package config

import (
	"strings"
	"testing"
)

func TestNormalizeVersioning(t *testing.T) {
	cfg := &Config{Version: "2.0", Versioning: &VersioningConfig{Strategy: "BRANCHES_ONLY", MaxVersionsPerRepo: -5, BranchPatterns: []string{"  main ", "", "release/*"}, TagPatterns: []string{"  v*"}}}
	res, err := NormalizeConfig(cfg)
	if err != nil {
		t.Fatalf("NormalizeConfig error: %v", err)
	}
	if cfg.Versioning.Strategy != StrategyBranchesOnly {
		t.Fatalf("strategy normalization failed: %v", cfg.Versioning.Strategy)
	}
	if cfg.Versioning.MaxVersionsPerRepo != 0 {
		t.Fatalf("expected clamp to 0 (unlimited), got %d", cfg.Versioning.MaxVersionsPerRepo)
	}
	if len(cfg.Versioning.BranchPatterns) != 2 || cfg.Versioning.BranchPatterns[0] != "main" {
		t.Fatalf("branch pattern trimming failed: %#v", cfg.Versioning.BranchPatterns)
	}
	if len(cfg.Versioning.TagPatterns) != 1 || cfg.Versioning.TagPatterns[0] != "v*" {
		t.Fatalf("tag pattern trimming failed: %#v", cfg.Versioning.TagPatterns)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("expected warnings (strategy change)")
	}
}

func TestNormalizeVersioningUnknownStrategy(t *testing.T) {
	cfg := &Config{Version: "2.0", Versioning: &VersioningConfig{Strategy: "mystery"}}
	res, err := NormalizeConfig(cfg)
	if err != nil {
		t.Fatalf("NormalizeConfig error: %v", err)
	}
	// Unknown strategy should be preserved for validation to reject (no silent coercion)
	if cfg.Versioning.Strategy != "mystery" {
		t.Fatalf("expected strategy preserved as 'mystery', got %v", cfg.Versioning.Strategy)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "invalid versioning.strategy") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected invalid strategy warning, got %v", res.Warnings)
	}
}
