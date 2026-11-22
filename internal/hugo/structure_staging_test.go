package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The staging directory is emptied before the build. It is left over by
// a build that was killed or that failed, and a page that is still in it
// would be published again.
func TestBeginStagingEmptiesTheDirectory(t *testing.T) {
	output := filepath.Join(t.TempDir(), "site")

	g := NewGenerator(&config.Config{}, output)
	if err := g.beginStaging(); err != nil {
		t.Fatalf("begin staging: %v", err)
	}

	stale := filepath.Join(g.StageDir(), "content", "old-repo", "guide.md")
	if err := os.MkdirAll(filepath.Dir(stale), 0o750); err != nil {
		t.Fatalf("make the stale file: %v", err)
	}
	if err := os.WriteFile(stale, []byte("# Old\n"), 0o600); err != nil {
		t.Fatalf("write the stale file: %v", err)
	}

	// The second build starts with the directory of the first one.
	if err := g.beginStaging(); err != nil {
		t.Fatalf("begin staging: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("the stale page is still in the staging directory: %v", err)
	}
}
