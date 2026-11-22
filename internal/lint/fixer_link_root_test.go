package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// The root of the search can be a file that the fixer has just
// renamed: the file is not there any more, and the search must still
// work. On a file system that keeps the case the stat of the old name
// still succeeds, and the bug hides there.
func TestFindLinksToFileWithARootThatIsGone(t *testing.T) {
	dir := t.TempDir()

	// A page that links to the file.
	page := filepath.Join(dir, "guide.md")
	content := "# Guide\n\nSee the [steps](Old-Name.md) for details.\n"
	if err := os.WriteFile(page, []byte(content), 0o600); err != nil {
		t.Fatalf("write the page: %v", err)
	}

	// The file the page links to was renamed and is not there.
	old := filepath.Join(dir, "Old-Name.md")

	fixer := NewFixer(nil, false, false)
	links, err := fixer.findLinksToFile(old, old)
	if err != nil {
		t.Fatalf("find the links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1", len(links))
	}
}
