package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/search"
)

// The index of the site holds the pages of the last build. Two builds
// that finish close together start two index jobs, and the slower one
// of them wrote the pages of the older build into the index of the
// newer one: the site held pages that are not published any more.
func TestTheIndexIsWrittenByOneJobAtATime(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("# Page\n"), 0o600); err != nil {
		t.Fatalf("write the page: %v", err)
	}

	index, err := search.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open the index: %v", err)
	}
	svc := &SearchService{index: index, maxResults: 10}

	// The lock is held: the job has to wait for it.
	svc.writeMu.Lock()

	done := make(chan struct{})
	go func() {
		svc.writeIndex(context.Background(), dir)
		close(done)
	}()

	select {
	case <-done:
		svc.writeMu.Unlock()
		t.Fatal("the index was written while another job held the lock")
	case <-time.After(200 * time.Millisecond):
	}

	svc.writeMu.Unlock()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the job did not write the index after the lock was released")
	}
}
