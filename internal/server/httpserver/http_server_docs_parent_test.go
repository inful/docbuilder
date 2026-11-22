package httpserver

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The URL path is the only user input that reaches the filesystem, so
// the walk for the nearest page has to stay under the docs root.
func TestFindNearestValidParentStaysUnderTheDocsRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "site")
	if err := os.Mkdir(root, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "guide"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "guide", "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Output: config.OutputConfig{
			Directory: root,
		},
		Daemon: &config.DaemonConfig{},
	}
	srv := New(cfg, testRuntime{}, Options{})

	// The nearest page above the missing one is found.
	if got := srv.findNearestValidParent(root, "/guide/missing/page"); got != "/guide/" {
		t.Errorf("findNearestValidParent = %q, want %q", got, "/guide/")
	}

	// A path that climbs out of the root resolves to nothing, even when
	// the parent directory holds a page.
	if got := srv.findNearestValidParent(root, "../index.html"); got != "/" {
		t.Errorf("findNearestValidParent = %q, want %q", got, "/")
	}
}
