package foundation

import (
	"os"
	"path/filepath"
	"testing"
)

// The file is written without a window in which it is half written: the
// data goes into a file next to it, and that file takes its place.
func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")

	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	got, err := os.ReadFile(path) // #nosec G304 -- the path is the test's own file
	if err != nil {
		t.Fatalf("read the file: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("the file holds %q", got)
	}

	// No temporary file is left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read the dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("the directory holds %d files, want 1", len(entries))
	}
}
