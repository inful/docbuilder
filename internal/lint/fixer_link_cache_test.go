package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// The fixer looks for the links to a renamed file once per renamed
// file. Reading and parsing every markdown file of the tree every time
// is the slow part, so the links of a file are read once and kept.
func TestFixer_ParsedLinksAreKeptAfterTheFirstRead(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.md")
	if err := os.WriteFile(file, []byte("[link](b.md)\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := &Fixer{linkCache: make(map[string]cachedLinks)}

	first, err := f.parsedLinks(file)
	if err != nil {
		t.Fatalf("parsed links: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("found %d links, want 1", len(first))
	}

	// The file is gone: the fixer still has its links, so it does not
	// read the tree again for every renamed file.
	if rmErr := os.Remove(file); rmErr != nil {
		t.Fatalf("remove file: %v", rmErr)
	}
	second, err := f.parsedLinks(file)
	if err != nil {
		t.Fatalf("the links were read again instead of kept: %v", err)
	}
	if len(second) != 1 || second[0].resolved != first[0].resolved {
		t.Fatalf("the kept links are %+v, want %+v", second, first)
	}
}

// A file that changed has to be read again, even when the fixer kept
// its links from an earlier run.
func TestFixer_ParsedLinksAreReadAgainAfterAChange(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.md")
	if err := os.WriteFile(file, []byte("[link](b.md)\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := &Fixer{linkCache: make(map[string]cachedLinks)}

	first, err := f.parsedLinks(file)
	if err != nil {
		t.Fatalf("parsed links: %v", err)
	}

	// A different length, so the change is visible in the file's size.
	if writeErr := os.WriteFile(file, []byte("[link](c.md)\n"), 0o600); writeErr != nil {
		t.Fatalf("rewrite file: %v", writeErr)
	}

	second, err := f.parsedLinks(file)
	if err != nil {
		t.Fatalf("parsed links: %v", err)
	}
	if second[0].resolved == first[0].resolved {
		t.Fatalf("the links were not read again after the file changed")
	}
}

// The cache outlives a run: the fixer looks for the links to a renamed
// file once per renamed file, and the same files are looked at again
// on the next run.
func TestFixer_LinkCacheOutlivesARun(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.md")
	if err := os.WriteFile(file, []byte("[link](b.md)\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := &Fixer{linkCache: make(map[string]cachedLinks)}

	if _, err := f.parsedLinks(file); err != nil {
		t.Fatalf("parsed links: %v", err)
	}
	entry, ok := f.linkCache[file]
	if !ok {
		t.Fatal("the file was not cached")
	}

	// A second look over the same tree keeps the entry.
	if _, err := f.parsedLinks(file); err != nil {
		t.Fatalf("parsed links: %v", err)
	}
	kept, ok := f.linkCache[file]
	if !ok || kept.modTime != entry.modTime || kept.size != entry.size {
		t.Fatalf("the cache entry was replaced: %+v, want %+v", kept, entry)
	}
}
