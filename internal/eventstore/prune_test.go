package eventstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// The event log must not grow without bound: it is pruned by age.
func TestSQLiteStore_PruneRemovesOldEvents(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	if err := store.Append(ctx, "old-build", "BuildStarted", []byte("{}"), nil); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := store.Append(ctx, "new-build", "BuildStarted", []byte("{}"), nil); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Prune everything older than a moment from now.
	removed, err := store.Prune(ctx, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 2 {
		t.Errorf("expected 2 events to be pruned, got %d", removed)
	}

	events, err := store.GetRange(ctx, time.Time{}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected the store to be empty after pruning, got %d events", len(events))
	}
}

// Pruning keeps the events that are still inside the window.
func TestSQLiteStore_PruneKeepsRecentEvents(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	if err := store.Append(ctx, "build", "BuildStarted", []byte("{}"), nil); err != nil {
		t.Fatalf("append: %v", err)
	}

	removed, err := store.Prune(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 0 {
		t.Errorf("expected nothing to be pruned, got %d", removed)
	}

	events, err := store.GetByBuildID(ctx, "build")
	if err != nil {
		t.Fatalf("by build id: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected the event to survive, got %d", len(events))
	}
}
