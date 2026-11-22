package linkverify

import (
	"os"
	"path/filepath"
	"testing"
)

// The front matter of a page is read from the source file on demand, so a
// build does not read every content file again. A broken link still needs
// it, so the read has to happen then.
func TestPageMetadata_FrontMatterReadOnDemand(t *testing.T) {
	dir := t.TempDir()
	contentPath := filepath.Join(dir, "page.md")
	body := "---\ntitle: The Page\ndescription: A page\n---\n# Body\n"
	if err := os.WriteFile(contentPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write content: %v", err)
	}

	page := &PageMetadata{ContentPath: contentPath}

	fm := page.frontMatter()
	if fm == nil {
		t.Fatal("expected the front matter to be read from the content file")
	}
	if title, _ := fm["title"].(string); title != "The Page" {
		t.Errorf("title: got %q want %q", title, "The Page")
	}

	// The second call uses the cached value.
	if err := os.Remove(contentPath); err != nil {
		t.Fatalf("remove content: %v", err)
	}
	fm = page.frontMatter()
	if title, _ := fm["title"].(string); title != "The Page" {
		t.Errorf("cached title: got %q want %q", title, "The Page")
	}
}

// A page with no content file simply has no front matter.
func TestPageMetadata_FrontMatterMissing(t *testing.T) {
	page := &PageMetadata{ContentPath: filepath.Join(t.TempDir(), "missing.md")}
	if fm := page.frontMatter(); fm != nil {
		t.Errorf("expected no front matter, got %v", fm)
	}
}
