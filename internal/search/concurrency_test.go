package search

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// The daemon serves search requests while a build writes the index.
// The index must hold up under that: a reader must not fail because a
// writer holds the lock, and a writer must not fail because another
// writer got there first.
func TestIndexHoldsUpUnderConcurrentUse(t *testing.T) {
	idx := openTestIndex(t)
	docs := make([]Document, 0, 500)
	for i := range 500 {
		docs = append(docs, Document{
			URL:   fmt.Sprintf("/page-%d/", i),
			Title: "Page",
			Body:  "hello world this is a page of the documentation",
		})
	}
	if err := idx.Upsert(context.Background(), docs); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 64)

	// Readers: the server answering queries.
	for range 8 {
		wg.Go(func() {
			for range 20 {
				if _, err := idx.Search(context.Background(), "hello", 10); err != nil {
					errs <- err
					return
				}
			}
		})
	}
	// Writers: builds finishing at the same time and rewriting the
	// index.
	for range 4 {
		wg.Go(func() {
			for range 5 {
				if err := idx.Upsert(context.Background(), docs); err != nil {
					errs <- err
					return
				}
			}
		})
	}

	wg.Wait()
	close(errs)

	failures := 0
	for err := range errs {
		failures++
		if failures <= 3 {
			t.Errorf("concurrent use failed: %v", err)
		}
	}
	if failures > 0 {
		t.Fatalf("%d concurrent operations failed", failures)
	}
}

// Every connection of the pool must wait for a lock instead of failing
// at once. A pragma set through a statement only reaches the one
// connection that ran it.
func TestEveryConnectionWaitsForTheLock(t *testing.T) {
	idx := openTestIndex(t)

	// Open more connections than the pool has, so the pool has to open
	// new ones.
	idx.db.SetMaxOpenConns(8)

	var wg sync.WaitGroup
	waits := make(chan int, 32)
	for range 8 {
		wg.Go(func() {
			conn, err := idx.db.Conn(context.Background())
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()

			var timeout int
			if err := conn.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&timeout); err != nil {
				return
			}
			waits <- timeout
		})
	}
	wg.Wait()
	close(waits)

	for timeout := range waits {
		if timeout == 0 {
			t.Fatal("a connection does not wait for the lock")
		}
	}
}
