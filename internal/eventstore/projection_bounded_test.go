package eventstore

import (
	"testing"
	"time"
)

// The rebuild must not replay the whole log: it reads the events of the
// recent builds only, and drops the build the window cut in half.
func TestBuildHistoryProjection_RebuildIsBounded(t *testing.T) {
	ctx := t.Context()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// An old build with more events than the rebuild reads.
	oldBuild := "build-old"
	for range maxRebuildEvents + 5 {
		event, _ := NewRepositoryCloned(oldBuild, "repo", "hash", "/path", time.Second)
		if err := store.Append(ctx, oldBuild, event.Type(), event.Payload(), nil); err != nil {
			t.Fatalf("failed to append: %v", err)
		}
	}

	// The newest build, with a complete story.
	newBuild := "build-new"
	started, _ := NewBuildStarted(newBuild, BuildStartedMeta{TenantID: "tenant-1"})
	if err := store.Append(ctx, newBuild, started.Type(), started.Payload(), nil); err != nil {
		t.Fatalf("failed to append: %v", err)
	}
	completed, _ := NewBuildCompleted(newBuild, "completed", time.Second, nil)
	if err := store.Append(ctx, newBuild, completed.Type(), completed.Payload(), nil); err != nil {
		t.Fatalf("failed to append: %v", err)
	}

	projection := NewBuildHistoryProjection(store, 10)
	if err := projection.Rebuild(ctx); err != nil {
		t.Fatalf("failed to rebuild: %v", err)
	}

	summary, exists := projection.GetBuild(newBuild)
	if !exists {
		t.Fatal("expected the newest build to exist after the rebuild")
	}
	if summary.Status != "completed" {
		t.Fatalf("status = %q, want %q", summary.Status, "completed")
	}
	if _, exists := projection.GetBuild(oldBuild); exists {
		t.Fatal("expected the cut build to be dropped")
	}
}
