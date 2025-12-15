package hugo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestHugoConfigGolden_TaxonomiesDefault(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Taxonomies Test Site",
			Theme: "hextra",
			// No custom taxonomies - should use defaults
		},
		Repositories: []config.Repository{
			{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}},
		},
	}

	g := NewGenerator(cfg, out)
	if err := g.generateHugoConfig(); err != nil {
		t.Fatalf("generate: %v", err)
	}

	actual := normalizeConfig(t, filepath.Join(out, "hugo.yaml"))
	golden := filepath.Join("testdata", "hugo_config", "taxonomies_default.yaml")

	// #nosec G304 - test file
	want, err := os.ReadFile(golden)
	if err != nil {
		// If golden file doesn't exist and UPDATE_GOLDEN=1, create it
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(golden, actual, 0o644); err != nil {
				t.Fatalf("write golden: %v", err)
			}
			t.Logf("Created golden file: %s", golden)
			return
		}
		t.Fatalf("read golden: %v", err)
	}

	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(actual)) {
		writeMismatch(t, want, actual)
		t.Fatal("config mismatch (see above)")
	}
}

func TestHugoConfigGolden_TaxonomiesCustom(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Custom Taxonomies Site",
			Theme: "hextra",
			Taxonomies: map[string]string{
				"tag":      "tags",
				"category": "categories",
				"author":   "authors",
				"topic":    "topics",
			},
		},
		Repositories: []config.Repository{
			{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}},
		},
	}

	g := NewGenerator(cfg, out)
	if err := g.generateHugoConfig(); err != nil {
		t.Fatalf("generate: %v", err)
	}

	actual := normalizeConfig(t, filepath.Join(out, "hugo.yaml"))
	golden := filepath.Join("testdata", "hugo_config", "taxonomies_custom.yaml")

	// #nosec G304 - test file
	want, err := os.ReadFile(golden)
	if err != nil {
		// If golden file doesn't exist and UPDATE_GOLDEN=1, create it
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(golden, actual, 0o644); err != nil {
				t.Fatalf("write golden: %v", err)
			}
			t.Logf("Created golden file: %s", golden)
			return
		}
		t.Fatalf("read golden: %v", err)
	}

	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(actual)) {
		writeMismatch(t, want, actual)
		t.Fatal("config mismatch (see above)")
	}
}
