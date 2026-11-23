package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// The file that the fixer rewrites keeps its mode. A document that
// another user could read before the fix has to stay readable after it.
func TestTheLinkUpdaterKeepsTheModeOfTheFile(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "index.md")

	if err := os.WriteFile(doc, []byte("# Index\n\n[Other](other.md)\n"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	// The mode of the file is the point of the test.
	//nolint:gosec // the mode of the file is the point of the test
	if err := os.Chmod(doc, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	f := &Fixer{linkCache: make(map[string]cachedLinks), rootPath: dir}
	links := []LinkReference{{SourceFile: doc, LineNumber: 3, Target: "other.md", FullMatch: "[Other](other.md)"}}

	if _, err := f.applyLinkUpdates(links, "other.md", "renamed.md"); err != nil {
		t.Fatalf("apply link updates: %v", err)
	}

	info, err := os.Stat(doc)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Fatalf("the mode of the file changed: %o", perm)
	}
}
