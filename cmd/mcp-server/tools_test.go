package main

import (
	"path/filepath"
	"testing"
)

func TestResolveInDocs_AcceptsRelativePath(t *testing.T) {
	docs := t.TempDir()
	got, err := resolveInDocs(docs, filepath.Join("guides", "intro.md"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(docs, "guides", "intro.md")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestResolveInDocs_AcceptsAbsoluteInside(t *testing.T) {
	docs := t.TempDir()
	target := filepath.Join(docs, "x.md")
	got, err := resolveInDocs(docs, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != target {
		t.Errorf("got %q want %q", got, target)
	}
}

func TestResolveInDocs_RejectsEscape(t *testing.T) {
	docs := t.TempDir()
	bad := filepath.Join(docs, "..", "escape.md")
	if _, err := resolveInDocs(docs, bad); err == nil {
		t.Fatalf("expected error for path-escape, got none")
	}
}

func TestResolveInDocs_RejectsSiblingDirectory(t *testing.T) {
	docs := t.TempDir()
	sibling := filepath.Join(filepath.Dir(docs), "outside.md")
	if _, err := resolveInDocs(docs, sibling); err == nil {
		t.Fatalf("expected error for sibling path, got none")
	}
}

func TestResolveInDocs_RejectsPrefixCollision(t *testing.T) {
	// /tmp/docs-evil must NOT pass a /tmp/docs containment check.
	parent := t.TempDir()
	docs := filepath.Join(parent, "docs")
	evil := filepath.Join(parent, "docs-evil", "x.md")
	if _, err := resolveInDocs(docs, evil); err == nil {
		t.Fatalf("expected error for prefix-collision, got none")
	}
}
