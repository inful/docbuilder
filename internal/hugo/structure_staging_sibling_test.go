package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The site is moved into its place with a rename. The staging
// directory is a sibling of the output: a rename across directories
// fails when they are on different file systems.
func TestTheStagingDirectoryIsASiblingOfTheOutput(t *testing.T) {
	base := t.TempDir()
	output := filepath.Join(t.TempDir(), "site")

	cfg := &config.Config{}
	cfg.Output.BaseDirectory = base
	cfg.Output.Directory = output

	g := NewGenerator(cfg, output)
	if err := g.beginStaging(); err != nil {
		t.Fatalf("begin staging: %v", err)
	}

	stage := g.StageDir()
	if filepath.Dir(stage) != filepath.Dir(output) {
		t.Fatalf("the staging directory %q is not next to the output %q", stage, output)
	}

	// The site is moved into its place.
	page := filepath.Join(stage, "content", "page.md")
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
