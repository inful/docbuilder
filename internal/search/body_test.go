package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The excerpt the reader sees must hold the words of the page, not the
// markup around them.
func TestTheExcerptHoldsTheWordsOfThePage(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "content")
	public := filepath.Join(dir, "public")
	if err := os.MkdirAll(filepath.Join(content, "repo", "install"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(public, "repo", "install"), 0o750); err != nil {
		t.Fatal(err)
	}
	page := "---\ntitle: Installing\n---\n\n# Installing the daemon\n\n" +
		"Install the daemon with the **package manager** on `linux`.\n\n" +
		"```bash\ndocbuilder install\n```\n"
	if err := os.WriteFile(filepath.Join(content, "repo", "install", "index.md"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(public, "repo", "install", "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	docs, err := Collect(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	for _, markup := range []string{"**", "`", "```", "# "} {
		if strings.Contains(docs[0].Body, markup) {
			t.Fatalf("the body still holds the markup %q: %q", markup, docs[0].Body)
		}
	}

	idx := openTestIndex(t)
	if err = idx.Upsert(context.Background(), docs); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	results, err := idx.Search(context.Background(), "package manager", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	for _, markup := range []string{"**", "`", "```"} {
		if strings.Contains(results[0].Excerpt, markup) {
			t.Fatalf("the excerpt still holds the markup %q: %q", markup, results[0].Excerpt)
		}
	}
	if !strings.Contains(results[0].Excerpt, "package manager") {
		t.Fatalf("the excerpt does not hold the words: %q", results[0].Excerpt)
	}
}
