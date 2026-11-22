package search

import (
	"os"
	"path/filepath"
	"testing"
)

// A page can ask for an address of its own with "url" in the front
// matter. It is then rendered there, so the index must find it there:
// a page that cannot be reached must not turn up in a search result,
// and a page that can must not go missing.
func TestCollectFindsAPageWithItsOwnAddress(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "content")
	public := filepath.Join(dir, "public")
	if err := os.MkdirAll(filepath.Join(content, "repo"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(public, "custom", "place"), 0o750); err != nil {
		t.Fatal(err)
	}
	page := "---\ntitle: Custom URL page\nurl: /custom/place/\n---\n\n" +
		"# Custom URL page\n\nA page that asks for an address of its own.\n"
	if err := os.WriteFile(filepath.Join(content, "repo", "custom.md"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(public, "custom", "place", "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	docs, err := Collect(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	if docs[0].URL != "/custom/place/" {
		t.Fatalf("url = %q, want %q", docs[0].URL, "/custom/place/")
	}
}

// A page that was not rendered cannot be reached, so it must not turn
// up in a search result.
func TestCollectSkipsAPageThatWasNotRendered(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "content")
	public := filepath.Join(dir, "public")
	if err := os.MkdirAll(filepath.Join(content, "repo"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(public, 0o750); err != nil {
		t.Fatal(err)
	}
	page := "---\ntitle: A page\n---\n\n# A page\n"
	if err := os.WriteFile(filepath.Join(content, "repo", "page.md"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}

	docs, err := Collect(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("got %d docs, want none", len(docs))
	}
}
