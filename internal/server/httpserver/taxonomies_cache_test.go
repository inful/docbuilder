package httpserver

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The taxonomies of the site are read from the files of the site. They
// are kept until the content directory changes: the build replaces the
// whole directory, so the time of the directory changes with it.
func TestTaxonomiesAreKeptUntilTheContentChanges(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "content", "repo")
	if err := os.MkdirAll(content, 0o750); err != nil {
		t.Fatalf("make the content dir: %v", err)
	}
	page := filepath.Join(content, "page.md")
	if err := os.WriteFile(page, []byte("---\ntitle: Page\ntags:\n  - first\n---\n"), 0o600); err != nil {
		t.Fatalf("write the page: %v", err)
	}

	srv := New(&config.Config{Output: config.OutputConfig{Directory: dir}}, testRuntime{}, Options{})

	tags, _, err := srv.siteTaxonomies(filepath.Join(dir, "content"))
	if err != nil {
		t.Fatalf("read the taxonomies: %v", err)
	}
	if len(tags) != 1 || tags[0] != "first" {
		t.Fatalf("the tags are %v", tags)
	}

	// The answer is kept: the files are not read again.
	if writeErr := os.WriteFile(page, []byte("---\ntitle: Page\ntags:\n  - second\n---\n"), 0o600); writeErr != nil {
		t.Fatalf("write the page: %v", writeErr)
	}
	kept, _, err := srv.siteTaxonomies(filepath.Join(dir, "content"))
	if err != nil {
		t.Fatalf("read the taxonomies: %v", err)
	}
	if len(kept) != 1 || kept[0] != "first" {
		t.Fatalf("the tags are %v, want the ones of the last read", kept)
	}

	// A build replaces the content directory, and the answer is read
	// again then.
	newContent := filepath.Join(dir, "content-new")
	if mkdirErr := os.MkdirAll(newContent, 0o750); mkdirErr != nil {
		t.Fatalf("make the new content dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(newContent, "page.md"), []byte("---\ntitle: Page\ntags:\n  - second\n---\n"), 0o600); writeErr != nil {
		t.Fatalf("write the page: %v", writeErr)
	}
	// The build moves the old directory away and puts the new one in
	// its place.
	if renameErr := os.Rename(filepath.Join(dir, "content"), filepath.Join(dir, "content-old")); renameErr != nil {
		t.Fatalf("move the content dir: %v", renameErr)
	}
	if renameErr := os.Rename(newContent, filepath.Join(dir, "content")); renameErr != nil {
		t.Fatalf("replace the content dir: %v", renameErr)
	}

	fresh, _, err := srv.siteTaxonomies(filepath.Join(dir, "content"))
	if err != nil {
		t.Fatalf("read the taxonomies: %v", err)
	}
	if len(fresh) != 1 || fresh[0] != "second" {
		t.Fatalf("the tags are %v, want the ones of the new content", fresh)
	}
}
