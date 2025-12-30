package config

import "testing"

// helper to build minimal config.
func baseCfg() *Config {
	return &Config{Version: "2.0", Hugo: HugoConfig{BaseURL: "https://example", Title: "Docs"}}
}

func TestSnapshotStableAcrossNormalizationVariants(t *testing.T) {
	a := baseCfg()
	a.Build.RenderMode = "AlWaYs" // mixed case
	a.Versioning = &VersioningConfig{Strategy: "BRANCHES_ONLY", BranchPatterns: []string{"main", "release/*"}, TagPatterns: []string{"v*"}}
	if _, err := NormalizeConfig(a); err != nil {
		t.Fatalf("normalize a: %v", err)
	}
	if err := applyDefaults(a); err != nil {
		t.Fatalf("defaults a: %v", err)
	}
	snapA := a.Snapshot()

	b := baseCfg()
	b.Build.RenderMode = "always"                                                                                                           // already canonical
	b.Versioning = &VersioningConfig{Strategy: "branches_only", BranchPatterns: []string{"release/*", "main"}, TagPatterns: []string{"v*"}} // different order
	if _, err := NormalizeConfig(b); err != nil {
		t.Fatalf("normalize b: %v", err)
	}
	if err := applyDefaults(b); err != nil {
		t.Fatalf("defaults b: %v", err)
	}
	snapB := b.Snapshot()

	if snapA != snapB {
		t.Fatalf("expected snapshots equal, got\nA=%s\nB=%s", snapA, snapB)
	}
}

func TestSnapshotDetectsMeaningfulChange(t *testing.T) {
	c := baseCfg()
	c.Build.RenderMode = RenderModeAuto
	if _, err := NormalizeConfig(c); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := applyDefaults(c); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	snap1 := c.Snapshot()
	c.Build.RenderMode = RenderModeAlways
	snap2 := c.Snapshot() // render_mode changed post-normalization is fine
	if snap1 == snap2 {
		t.Fatalf("expected snapshot change after render_mode modification")
	}
}
