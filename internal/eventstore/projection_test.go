package eventstore

import (
	"context"
	"testing"
	"time"
)

func TestBuildHistoryProjection_ApplyEvents(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	projection := NewBuildHistoryProjection(store, 10)

	// Apply BuildStarted event
	buildID := "build-123"
	startEvent, err := NewBuildStarted(buildID, BuildStartedMeta{TenantID: "tenant-1", Type: "manual"})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}
	projection.Apply(startEvent)

	// Check build is tracked
	summary, exists := projection.GetBuild(buildID)
	if !exists {
		t.Fatal("Expected build to exist")
	}
	if summary.Status != "running" {
		t.Errorf("Expected status 'running', got %q", summary.Status)
	}
	if summary.TenantID != "tenant-1" {
		t.Errorf("Expected tenant 'tenant-1', got %q", summary.TenantID)
	}

	// Apply RepositoryCloned event
	cloneEvent, err := NewRepositoryCloned(buildID, "repo1", "abc123", "/tmp/repo1", time.Second)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}
	projection.Apply(cloneEvent)

	summary, _ = projection.GetBuild(buildID)
	if summary.RepoCount != 1 {
		t.Errorf("Expected repo count 1, got %d", summary.RepoCount)
	}

	// Apply DocumentsDiscovered event
	discoverEvent, err := NewDocumentsDiscovered(buildID, "repo1", []string{"doc1.md", "doc2.md", "doc3.md"})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}
	projection.Apply(discoverEvent)

	summary, _ = projection.GetBuild(buildID)
	if summary.FileCount != 3 {
		t.Errorf("Expected file count 3, got %d", summary.FileCount)
	}

	// Apply BuildCompleted event
	completeEvent, err := NewBuildCompleted(buildID, "completed", 5*time.Second, map[string]string{"site": "/output"})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}
	projection.Apply(completeEvent)

	summary, _ = projection.GetBuild(buildID)
	if summary.Status != "completed" {
		t.Errorf("Expected status 'completed', got %q", summary.Status)
	}
	if summary.CompletedAt == nil {
		t.Error("Expected completed_at to be set")
	}
	if summary.Artifacts["site"] != "/output" {
		t.Errorf("Expected artifact 'site' = '/output', got %q", summary.Artifacts["site"])
	}

	// Check history
	history := projection.GetHistory()
	if len(history) != 1 {
		t.Fatalf("Expected 1 history entry, got %d", len(history))
	}
	if history[0].BuildID != buildID {
		t.Errorf("Expected build ID %q, got %q", buildID, history[0].BuildID)
	}
}

func TestBuildHistoryProjection_BuildFailed(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	projection := NewBuildHistoryProjection(store, 10)

	buildID := "build-failed"
	startEvent, _ := NewBuildStarted(buildID, BuildStartedMeta{})
	projection.Apply(startEvent)

	failEvent, _ := NewBuildFailed(buildID, "clone", "git auth failed")
	projection.Apply(failEvent)

	summary, exists := projection.GetBuild(buildID)
	if !exists {
		t.Fatal("Expected build to exist")
	}
	if summary.Status != "failed" {
		t.Errorf("Expected status 'failed', got %q", summary.Status)
	}
	if summary.ErrorStage != "clone" {
		t.Errorf("Expected error stage 'clone', got %q", summary.ErrorStage)
	}
	if summary.ErrorMessage != "git auth failed" {
		t.Errorf("Expected error message 'git auth failed', got %q", summary.ErrorMessage)
	}
}

func TestBuildHistoryProjection_Rebuild(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Store some events directly
	buildID := "build-rebuild-test"
	startEvent, _ := NewBuildStarted(buildID, BuildStartedMeta{TenantID: "tenant-2"})
	if err := store.Append(ctx, buildID, startEvent.Type(), startEvent.Payload(), nil); err != nil {
		t.Fatalf("Failed to append: %v", err)
	}

	cloneEvent, _ := NewRepositoryCloned(buildID, "repo", "hash", "/path", time.Second)
	if err := store.Append(ctx, buildID, cloneEvent.Type(), cloneEvent.Payload(), nil); err != nil {
		t.Fatalf("Failed to append: %v", err)
	}

	completeEvent, _ := NewBuildCompleted(buildID, "completed", 3*time.Second, nil)
	if err := store.Append(ctx, buildID, completeEvent.Type(), completeEvent.Payload(), nil); err != nil {
		t.Fatalf("Failed to append: %v", err)
	}

	// Create a fresh projection and rebuild from store
	projection := NewBuildHistoryProjection(store, 10)
	if err := projection.Rebuild(ctx); err != nil {
		t.Fatalf("Failed to rebuild: %v", err)
	}

	// Verify the projection state
	summary, exists := projection.GetBuild(buildID)
	if !exists {
		t.Fatal("Expected build to exist after rebuild")
	}
	if summary.Status != "completed" {
		t.Errorf("Expected status 'completed', got %q", summary.Status)
	}
	if summary.RepoCount != 1 {
		t.Errorf("Expected repo count 1, got %d", summary.RepoCount)
	}

	// Verify history
	history := projection.GetHistory()
	if len(history) != 1 {
		t.Fatalf("Expected 1 history entry, got %d", len(history))
	}
}

func TestBuildHistoryProjection_HistoryLimit(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create projection with small max size
	projection := NewBuildHistoryProjection(store, 3)

	// Add 5 completed builds
	for i := 0; i < 5; i++ {
		buildID := "build-" + string(rune('a'+i))
		startEvent, _ := NewBuildStarted(buildID, BuildStartedMeta{})
		projection.Apply(startEvent)

		completeEvent, _ := NewBuildCompleted(buildID, "completed", time.Second, nil)
		projection.Apply(completeEvent)
	}

	// History should be limited to 3
	history := projection.GetHistory()
	if len(history) != 3 {
		t.Errorf("Expected history length 3, got %d", len(history))
	}
}

func TestBuildHistoryProjection_GetActiveBuild(t *testing.T) {
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	projection := NewBuildHistoryProjection(store, 10)

	// No active build initially
	active := projection.GetActiveBuild()
	if active != nil {
		t.Error("Expected no active build initially")
	}

	// Start a build
	startEvent, _ := NewBuildStarted("active-build", BuildStartedMeta{})
	projection.Apply(startEvent)

	active = projection.GetActiveBuild()
	if active == nil {
		t.Fatal("Expected active build")
	}
	if active.BuildID != "active-build" {
		t.Errorf("Expected build ID 'active-build', got %q", active.BuildID)
	}

	// Complete the build
	completeEvent, _ := NewBuildCompleted("active-build", "completed", time.Second, nil)
	projection.Apply(completeEvent)

	active = projection.GetActiveBuild()
	if active != nil {
		t.Error("Expected no active build after completion")
	}
}
