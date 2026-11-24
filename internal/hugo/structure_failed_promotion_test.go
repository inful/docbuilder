package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A promotion that fails must not cost the published site. The site is
// moved aside first and put back when the promotion fails - a build
// that breaks must not leave the daemon with nothing to serve.
func TestAFailedPromotionKeepsThePublishedSite(t *testing.T) {
	base := t.TempDir()
	output := filepath.Join(base, "site")

	cfg := &config.Config{}
	g := NewGenerator(cfg, output)

	// The published site of the last build.
	if err := os.MkdirAll(filepath.Join(output, "content"), 0o750); err != nil {
		t.Fatalf("make the output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(output, "content", "page.md"), []byte("# Page\n"), 0o600); err != nil {
		t.Fatalf("write the page: %v", err)
	}

	if err := g.beginStaging(); err != nil {
		t.Fatalf("begin staging: %v", err)
	}
	page := filepath.Join(g.StageDir(), "content", "page.md")
	if err := os.MkdirAll(filepath.Dir(page), 0o750); err != nil {
		t.Fatalf("make the page: %v", err)
	}
	if err := os.WriteFile(page, []byte("# New page\n"), 0o600); err != nil {
		t.Fatalf("write the page: %v", err)
	}

	// The staging directory is gone: the promotion cannot work.
	if err := os.RemoveAll(g.StageDir()); err != nil {
		t.Fatalf("remove the staging: %v", err)
	}

	if err := g.finalizeStaging(); err == nil {
		t.Fatal("the promotion of a missing staging directory succeeded")
	}

	// The site of the last build is still there.
	// #nosec G304 -- the path is built by the test itself.
	published, err := os.ReadFile(filepath.Join(output, "content", "page.md"))
	if err != nil {
		t.Fatalf("the published site is gone: %v", err)
	}
	if string(published) != "# Page\n" {
		t.Fatalf("the published site changed: %q", published)
	}
}
