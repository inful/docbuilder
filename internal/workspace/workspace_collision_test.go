package workspace

import (
	"os"
	"testing"
)

// Two workspaces must not share a directory. They are named after the
// second they were made in, so two builds that start in the same
// second would share one: the first one to finish would delete the
// workspace of the other while it clones, and that build would publish
// a site without pages.
func TestManager_EphemeralWorkspacesDoNotShareADirectory(t *testing.T) {
	base := t.TempDir()

	first := NewManager(base)
	if err := first.Create(); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	second := NewManager(base)
	if err := second.Create(); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if first.GetPath() == second.GetPath() {
		t.Fatalf("two workspaces share the directory %q", first.GetPath())
	}

	if err := first.Cleanup(); err != nil {
		t.Fatalf("Cleanup() failed: %v", err)
	}
	if _, err := os.Stat(second.GetPath()); err != nil {
		t.Fatalf("the cleanup of one workspace removed the other: %v", err)
	}
}
