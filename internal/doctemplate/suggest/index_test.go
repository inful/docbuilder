package suggest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestBuildFromDocsAndSuggest(t *testing.T) {
	docsDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(docsDir, "adr"))
	mustMkdirAll(t, filepath.Join(docsDir, "guides"))
	mustWrite(t, filepath.Join(docsDir, "adr", "adr-001-foo.md"), "# ADR")
	mustWrite(t, filepath.Join(docsDir, "guides", "getting-started.md"), "# Guide")
	mustWrite(t, filepath.Join(docsDir, "README.md"), "# Root")

	idx, err := BuildFromDocs(docsDir)
	if err != nil {
		t.Fatalf("BuildFromDocs failed: %v", err)
	}

	gotPrefix := idx.Suggest("adr", 10)
	if len(gotPrefix) == 0 || gotPrefix[0] != "adr" {
		t.Fatalf("expected adr prefix match first, got %v", gotPrefix)
	}

	gotContains := idx.Suggest("start", 10)
	if !slices.Contains(gotContains, "guides/getting-started") {
		t.Fatalf("expected contains match for guides/getting-started, got %v", gotContains)
	}
}

func TestSuggestLimit(t *testing.T) {
	idx := Index{candidates: []string{"alpha", "beta", "gamma"}}
	got := idx.Suggest("", 2)
	if len(got) != 2 {
		t.Fatalf("expected limit=2 results, got %d", len(got))
	}
}

func TestSuggestFromGlob_FileStem(t *testing.T) {
	docsDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(docsDir, "dir"))
	mustWrite(t, filepath.Join(docsDir, "dir", "something.md"), "# one")
	mustWrite(t, filepath.Join(docsDir, "dir", "another.md"), "# two")

	idx, err := BuildFromDocs(docsDir)
	if err != nil {
		t.Fatalf("BuildFromDocs failed: %v", err)
	}

	got := idx.SuggestFromGlob("dir/*.md", "som", 10)
	if !slices.Contains(got, "something") {
		t.Fatalf("expected glob stem suggestion 'something', got %v", got)
	}
}

func TestSuggestFromGlob_Directories(t *testing.T) {
	docsDir := t.TempDir()
	mustWrite(t, filepath.Join(docsDir, "dir", "one", "two.md"), "# two")
	mustWrite(t, filepath.Join(docsDir, "dir", "one", "three.md"), "# three")
	mustWrite(t, filepath.Join(docsDir, "dir", "subdir", "four.md"), "# four")
	mustWrite(t, filepath.Join(docsDir, "dir", "file.md"), "# file")

	idx, err := BuildFromDocs(docsDir)
	if err != nil {
		t.Fatalf("BuildFromDocs failed: %v", err)
	}

	got := idx.SuggestFromGlob("dir/*/", "", 10)
	if !slices.Contains(got, "one") || !slices.Contains(got, "subdir") {
		t.Fatalf("expected directory suggestions one and subdir, got %v", got)
	}
	if slices.Contains(got, "file") {
		t.Fatalf("did not expect file stem in directory suggestions, got %v", got)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir parent %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
