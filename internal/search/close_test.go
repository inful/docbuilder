package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The daemon closes the index when it stops. The write-ahead log is
// only folded back into the index when the last connection goes, so
// closing is what keeps the state directory tidy.
func TestCloseReleasesTheFiles(t *testing.T) {
	dir := t.TempDir()
	idx, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := idx.Upsert(context.Background(), []Document{{URL: "/a/", Title: "A", Body: "x"}}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := idx.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "search.db-wal")); err == nil {
		t.Fatal("the write-ahead log is still there: the index was not closed")
	}
	if _, err := os.Stat(filepath.Join(dir, "search.db-shm")); err == nil {
		t.Fatal("the shared memory file is still there: the index was not closed")
	}
	if _, err := os.Stat(filepath.Join(dir, "search.db")); err != nil {
		t.Fatalf("the index itself must survive the close: %v", err)
	}
}
