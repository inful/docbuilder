package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestCopyTaxonomyLayouts_WithTaxonomies(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Hugo.Taxonomies = map[string]string{
		"tag":      "tags",
		"category": "categories",
	}

	gen := NewGenerator(cfg, dir)
	if err := gen.copyTaxonomyLayouts(); err != nil {
		t.Fatalf("copyTaxonomyLayouts: %v", err)
	}

	// Relearn theme provides its own taxonomy layouts, so we should NOT copy any
	termsPath := filepath.Join(dir, "layouts", "_default", "terms.html")
	if _, err := os.Stat(termsPath); err == nil {
		t.Error("expected NO layouts/_default/terms.html for Relearn theme (uses built-in)")
	}
}

func TestCopyTaxonomyLayouts_WithoutTaxonomies(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	// No taxonomies configured (but layouts should still be copied for defaults)

	gen := NewGenerator(cfg, dir)
	if err := gen.copyTaxonomyLayouts(); err != nil {
		t.Fatalf("copyTaxonomyLayouts: %v", err)
	}

	// Layouts SHOULD be created even when user hasn't configured taxonomies
	// Relearn theme provides its own taxonomy layouts, verify we don't copy anything
	termsPath := filepath.Join(dir, "layouts", "_default", "terms.html")
	if _, err := os.Stat(termsPath); err == nil {
		t.Error("expected NO layouts for Relearn theme (uses built-in)")
	}
}

func TestCopyTaxonomyLayouts_NilConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	// Empty config with no taxonomies
	gen := NewGenerator(cfg, dir)

	// Should not error with empty config
	if err := gen.copyTaxonomyLayouts(); err != nil {
		t.Fatalf("copyTaxonomyLayouts with empty config: %v", err)
	}
}
