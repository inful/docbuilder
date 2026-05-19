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

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
