package git

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// The daemon saves the cache from two places: the status job and the
// handler of a removed repository. Two saves write the same temporary
// file and rename it over the cache: one of them renames the file the
// other is about to rename.
func TestTheRemoteHeadCacheSurvivesConcurrentSaves(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewRemoteHeadCache(dir)
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}

	// The saves start at the same time: the race is in the write and
	// the rename of the temporary file.
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cache.Set("https://example.com/repo.git", "main", "hash")
			<-start
			if err := cache.Save(); err != nil {
				t.Errorf("save %d: %v", i, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	// The cache on the disk is readable.
	if _, err := os.Stat(filepath.Join(dir, "remote-heads.json")); err != nil {
		t.Fatalf("the cache was not written: %v", err)
	}
}
