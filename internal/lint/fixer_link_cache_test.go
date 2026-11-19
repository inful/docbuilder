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

	f := &Fixer{linkCache: make(map[string][]resolvedLink)}

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

// A file that the fixer writes has changed, so its links have to be
// read again.
func TestFixer_ParsedLinksAreReadAgainAfterAWrite(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.md")
	if err := os.WriteFile(file, []byte("[link](b.md)\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	f := &Fixer{linkCache: make(map[string][]resolvedLink)}

	first, err := f.parsedLinks(file)
	if err != nil {
		t.Fatalf("parsed links: %v", err)
	}

	if writeErr := os.WriteFile(file, []byte("[link](c.md)\n"), 0o600); writeErr != nil {
		t.Fatalf("rewrite file: %v", writeErr)
	}
	delete(f.linkCache, file)

	second, err := f.parsedLinks(file)
	if err != nil {
		t.Fatalf("parsed links: %v", err)
	}
	if second[0].resolved == first[0].resolved {
		t.Fatalf("the links were not read again after the file was written")
	}
}
