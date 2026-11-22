package search

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSite writes a small built site: the content tree and the
// rendered tree.
func writeSite(t *testing.T, files map[string]string) string {
	t.Helper()
	site := t.TempDir()

	for rel, content := range files {
		contentPath := filepath.Join(site, "content", rel)
		if err := os.MkdirAll(filepath.Dir(contentPath), 0o750); err != nil {
			t.Fatalf("mkdir content: %v", err)
		}
		if err := os.WriteFile(contentPath, []byte(content), 0o600); err != nil {
			t.Fatalf("write content: %v", err)
		}
	}

	for rel := range files {
		rendered := renderedPathFor(rel)
		renderedPath := filepath.Join(site, "public", rendered)
		if err := os.MkdirAll(filepath.Dir(renderedPath), 0o750); err != nil {
			t.Fatalf("mkdir public: %v", err)
		}
		if err := os.WriteFile(renderedPath, []byte("<html></html>"), 0o600); err != nil {
			t.Fatalf("write public: %v", err)
		}
	}
	return site
}

// renderedPathFor returns the path a content file is rendered at.
func renderedPathFor(rel string) string {
	base := filepath.ToSlash(rel)
	base = base[:len(base)-len(".md")]
	if base == "_index" {
		return "index.html"
	}
	if filepath.Base(base) == "_index" {
		return filepath.ToSlash(filepath.Dir(base)) + "/index.html"
	}
	return base + "/index.html"
}

// The collected documents must carry the text, the title and the
// taxonomy of the page, and the address it is served at.
func TestCollectReadsThePagesOfASite(t *testing.T) {
	site := writeSite(t, map[string]string{
		"_index.md":      "---\ntitle: Documentation Portal\n---\n\nThe portal.\n",
		"adr/adr-001.md": "---\ntitle: 'ADR-001: Staging'\ncategories:\n  - architecture-decisions\ntags:\n  - adr\n  - staging\n---\n\nThe build renders into a staging directory.\n",
	})

	docs, err := Collect(site)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("collected %d documents, want 2: %+v", len(docs), docs)
	}

	byURL := map[string]Document{}
	for _, doc := range docs {
		byURL[doc.URL] = doc
	}

	adr, ok := byURL["/adr/adr-001/"]
	if !ok {
		t.Fatalf("the ADR page is missing: %+v", byURL)
	}
	if adr.Title != "ADR-001: Staging" {
		t.Fatalf("title = %q", adr.Title)
	}
	if adr.Categories != "architecture-decisions" {
		t.Fatalf("categories = %q", adr.Categories)
	}
	if adr.Tags != "adr staging" {
		t.Fatalf("tags = %q", adr.Tags)
	}
	if adr.Body != "The build renders into a staging directory." {
		t.Fatalf("body = %q", adr.Body)
	}

	if _, ok := byURL["/"]; !ok {
		t.Fatalf("the home page is missing: %+v", byURL)
	}
}

// A page that was not rendered cannot be reached, so it must not show
// up in a search result.
func TestCollectSkipsPagesThatWereNotRendered(t *testing.T) {
	site := t.TempDir()
	contentPath := filepath.Join(site, "content", "hidden.md")
	if err := os.MkdirAll(filepath.Dir(contentPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(contentPath, []byte("---\ntitle: Hidden\n---\n\nText.\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	// No public directory: nothing was rendered.

	docs, err := Collect(site)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("collected %d documents, want none: %+v", len(docs), docs)
	}
}

// A page that asks not to be searched is left out.
func TestCollectSkipsPagesThatOptOut(t *testing.T) {
	site := writeSite(t, map[string]string{
		"not-searched.md": "---\ntitle: Not searched\nexclude_search: true\n---\n\nHidden text.\n",
		"public.md":       "---\ntitle: Public\n---\n\nVisible text.\n",
	})

	docs, err := Collect(site)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("collected %d documents, want 1: %+v", len(docs), docs)
	}
	if docs[0].URL != "/public/" {
		t.Fatalf("collected %q, want the public page", docs[0].URL)
	}
}

// A page without a title gets one from its address.
func TestCollectFallsBackToTheAddressForTheTitle(t *testing.T) {
	site := writeSite(t, map[string]string{
		"how-to/install-the-daemon.md": "---\ncategories:\n  - how-to\n---\n\nText.\n",
	})

	docs, err := Collect(site)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("collected %d documents, want 1", len(docs))
	}
	if docs[0].Title != "Install the daemon" {
		t.Fatalf("title = %q", docs[0].Title)
	}
}
