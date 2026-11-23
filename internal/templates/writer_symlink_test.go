package templates

import (
	"os"
	"path/filepath"
	"testing"
)

// A directory of the docs that is a link to another place must not
// take the written file outside of the docs directory.
func TestWriteGeneratedFileStaysInTheDocsDirectory(t *testing.T) {
	base := t.TempDir()
	docs := filepath.Join(base, "docs")
	if err := os.MkdirAll(docs, 0o750); err != nil {
		t.Fatalf("make the docs dir: %v", err)
	}

	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatalf("make the outside dir: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(docs, "adr")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}

	if _, err := WriteGeneratedFile(docs, "adr/adr-001.md", "# ADR\n"); err == nil {
		t.Fatal("the file was written outside of the docs directory")
	}
}
