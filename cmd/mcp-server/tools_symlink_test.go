package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A symlink inside the docs directory that points outside it must not
// give access to the file it points at. The tools advertise that they
// stay inside the docs directory.
func TestResolveInDocs_RejectsASymlinkOutOfTheDirectory(t *testing.T) {
	docs := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("# Secret\n"), 0o600); err != nil {
		t.Fatalf("write the file outside: %v", err)
	}

	link := filepath.Join(docs, "link.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}

	if _, err := resolveInDocs(docs, "link.md"); err == nil {
		t.Fatal("expected the symlink to be refused, got none")
	}
}

// A directory that is a symlink to a directory outside the docs
// directory must be refused as well.
func TestResolveInDocs_RejectsASymlinkedDirectory(t *testing.T) {
	docs := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("# Secret\n"), 0o600); err != nil {
		t.Fatalf("write the file outside: %v", err)
	}

	link := filepath.Join(docs, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}

	if _, err := resolveInDocs(docs, filepath.Join("escape", "secret.md")); err == nil {
		t.Fatal("expected the symlinked directory to be refused, got none")
	}
}

// A file inside the docs directory is still allowed, also when it does
// not exist yet: a doc is written from nothing.
func TestResolveInDocs_AcceptsANewFile(t *testing.T) {
	docs := t.TempDir()

	got, err := resolveInDocs(docs, "new-doc.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(docs, "new-doc.md")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
