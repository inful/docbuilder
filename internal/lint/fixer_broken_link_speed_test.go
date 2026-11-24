package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Every broken link reads the directory of its target. A documentation
// set in one directory turns the check quadratic: 3000 documents with
// one broken link each read the listing of 3000 entries 3000 times.
func TestTheBrokenLinkCheckDoesNotReadTheDirectoryAgainAndAgain(t *testing.T) {
	dir := t.TempDir()
	for i := range 2000 {
		name := filepath.Join(dir, fmt.Sprintf("doc-%04d.md", i))
		if err := os.WriteFile(name, []byte("# Doc\n"), 0o600); err != nil {
			t.Fatalf("write the document: %v", err)
		}
	}

	forgetDirectoryListings()

	start := time.Now()
	for range 2000 {
		if fileExists(filepath.Join(dir, "missing.md")) {
			t.Fatal("the missing file was found")
		}
	}
	elapsed := time.Since(start)

	// The listing of the directory is read once: 2000 entries 2000
	// times take seconds.
	if elapsed > 2*time.Second {
		t.Fatalf("the check of 300 broken links took %s", elapsed)
	}
}
