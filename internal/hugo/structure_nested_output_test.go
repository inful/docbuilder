package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The output directory may be nested under the base directory: the
// directory that holds it has to be made before the site can be moved
// into it.
func TestFinalizeStagingCreatesTheParentOfTheOutputDirectory(t *testing.T) {
	base := t.TempDir()
	output := filepath.Join(base, "nested", "site")

	cfg := &config.Config{}
	cfg.Output.BaseDirectory = base
	cfg.Output.Directory = "nested/site"

	g := NewGenerator(cfg, output)
	if err := g.beginStaging(); err != nil {
		t.Fatalf("begin staging: %v", err)
	}

	page := filepath.Join(g.StageDir(), "content", "page.md")
	if err := os.MkdirAll(filepath.Dir(page), 0o750); err != nil {
		t.Fatalf("make the page: %v", err)
	}
	if err := os.WriteFile(page, []byte("# Page\n"), 0o600); err != nil {
		t.Fatalf("write the page: %v", err)
	}

	if err := g.finalizeStaging(); err != nil {
		t.Fatalf("finalize staging: %v", err)
	}

	if _, err := os.Stat(filepath.Join(output, "content", "page.md")); err != nil {
		t.Fatalf("the page was not published: %v", err)
	}
}
