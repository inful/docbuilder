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
	cfg.Hugo.Theme = "hextra"
	cfg.Hugo.Taxonomies = map[string]string{
		"tag":      "tags",
		"category": "categories",
	}

	gen := NewGenerator(cfg, dir)
	if err := gen.copyTaxonomyLayouts(); err != nil {
		t.Fatalf("copyTaxonomyLayouts: %v", err)
	}

	// Verify terms.html layout exists
	termsPath := filepath.Join(dir, "layouts", "_default", "terms.html")
	if _, err := os.Stat(termsPath); os.IsNotExist(err) {
		t.Error("expected layouts/_default/terms.html to exist")
	}

	// Verify term.html layout exists
	termPath := filepath.Join(dir, "layouts", "_default", "term.html")
	if _, err := os.Stat(termPath); os.IsNotExist(err) {
		t.Error("expected layouts/_default/term.html to exist")
	}

	// Verify content of terms.html
	termsContent, err := os.ReadFile(termsPath)
	if err != nil {
		t.Fatalf("failed to read terms.html: %v", err)
	}
	if len(termsContent) == 0 {
		t.Error("terms.html is empty")
	}

	// Verify content of term.html
	termContent, err := os.ReadFile(termPath)
	if err != nil {
		t.Fatalf("failed to read term.html: %v", err)
	}
	if len(termContent) == 0 {
		t.Error("term.html is empty")
	}
}

func TestCopyTaxonomyLayouts_WithoutTaxonomies(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Hugo.Theme = "hextra"
	// No taxonomies configured (but layouts should still be copied for defaults)

	gen := NewGenerator(cfg, dir)
	if err := gen.copyTaxonomyLayouts(); err != nil {
		t.Fatalf("copyTaxonomyLayouts: %v", err)
	}

	// Layouts SHOULD be created even when user hasn't configured taxonomies
	// because we always use default taxonomies (tags, categories)
	termsPath := filepath.Join(dir, "layouts", "_default", "terms.html")
	if _, err := os.Stat(termsPath); os.IsNotExist(err) {
		t.Error("expected layouts/_default/terms.html to exist for default taxonomies")
	}

	termPath := filepath.Join(dir, "layouts", "_default", "term.html")
	if _, err := os.Stat(termPath); os.IsNotExist(err) {
		t.Error("expected layouts/_default/term.html to exist for default taxonomies")
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
