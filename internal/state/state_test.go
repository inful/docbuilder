package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// TestJSONStore demonstrates basic functionality of the new state management system.
func TestJSONStore(t *testing.T) {
	t.Run("Repository Operations", testRepositoryOperations)
	t.Run("Build Operations", testBuildOperations)
	t.Run("Statistics Operations", testStatisticsOperations)
	t.Run("Transaction Operations", testTransactionOperations)
	t.Run("Health Check", testHealthCheck)
	t.Run("Persistence", testPersistence)
}

func testRepositoryOperations(t *testing.T) {
	store := createTestStore(t)
	ctx := t.Context()
	repoStore := store.Repositories()

	// Create a repository
	repo := &Repository{
		URL:    "https://github.com/test/repo.git",
		Name:   "test-repo",
		Branch: "main",
	}

	createResult := repoStore.Create(ctx, repo)
	if createResult.IsErr() {
		t.Fatalf("Failed to create repository: %v", createResult.UnwrapErr())
	}

	// Retrieve the repository
	getResult := repoStore.GetByURL(ctx, repo.URL)
	if getResult.IsErr() {
		t.Fatalf("Failed to get repository: %v", getResult.UnwrapErr())
	}

	if getResult.Unwrap().IsNone() {
		t.Fatal("Repository not found after creation")
	}

	retrieved := getResult.Unwrap().Unwrap()
	if retrieved.Name != repo.Name {
		t.Errorf("Expected name %q, got %q", repo.Name, retrieved.Name)
	}

	// List repositories
	listResult := repoStore.List(ctx)
	if listResult.IsErr() {
		t.Fatalf("Failed to list repositories: %v", listResult.UnwrapErr())
	}

	repos := listResult.Unwrap()
	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}

	// Increment build count
	incrementResult := repoStore.IncrementBuildCount(ctx, repo.URL, true)
	if incrementResult.IsErr() {
		t.Fatalf("Failed to increment build count: %v", incrementResult.UnwrapErr())
	}

	// Verify build count increased
	getResult = repoStore.GetByURL(ctx, repo.URL)
	if getResult.IsErr() {
		t.Fatalf("Failed to get repository after increment: %v", getResult.UnwrapErr())
	}

	updated := getResult.Unwrap().Unwrap()
	if updated.BuildCount != 1 {
		t.Errorf("Expected build count 1, got %d", updated.BuildCount)
	}

	// Update repository metadata for existing record
	if hashResult := repoStore.SetDocFilesHash(ctx, repo.URL, "abc123"); hashResult.IsErr() {
		t.Fatalf("Failed to set doc files hash: %v", hashResult.UnwrapErr())
	}
	if pathsResult := repoStore.SetDocFilePaths(ctx, repo.URL, []string{"docs/a.md", "docs/b.md"}); pathsResult.IsErr() {
		t.Fatalf("Failed to set doc file paths: %v", pathsResult.UnwrapErr())
	}

	metaResult := repoStore.GetByURL(ctx, repo.URL)
	if metaResult.IsErr() || metaResult.Unwrap().IsNone() {
		t.Fatalf("Failed to reload repository for metadata checks: %v", metaResult.UnwrapErr())
	}
	meta := metaResult.Unwrap().Unwrap()
	if meta.DocFilesHash.IsNone() || meta.DocFilesHash.Unwrap() != "abc123" {
		t.Fatalf("Doc files hash not stored correctly: %+v", meta.DocFilesHash)
	}
	if len(meta.DocFilePaths) != 2 || meta.DocFilePaths[0] != "docs/a.md" || meta.DocFilePaths[1] != "docs/b.md" {
		t.Fatalf("Doc file paths not stored correctly: %+v", meta.DocFilePaths)
	}

	t.Run("Repository metadata requires existing repo", func(t *testing.T) {
		testRepositoryMetadataValidation(t, repoStore)
	})
}

func testRepositoryMetadataValidation(t *testing.T, repoStore RepositoryStore) {
	t.Helper()
	ctx := t.Context()
	missingURL := "https://github.com/example/missing.git"
	errCases := []struct {
		name string
		call func() foundation.Result[struct{}, error]
	}{
		{
			name: "increment",
			call: func() foundation.Result[struct{}, error] {
				return repoStore.IncrementBuildCount(ctx, missingURL, true)
			},
		},
		{
			name: "doc-count",
			call: func() foundation.Result[struct{}, error] {
				return repoStore.SetDocumentCount(ctx, missingURL, 1)
			},
		},
		{
			name: "doc-hash",
			call: func() foundation.Result[struct{}, error] {
				return repoStore.SetDocFilesHash(ctx, missingURL, "hash")
			},
		},
		{
			name: "doc-paths",
			call: func() foundation.Result[struct{}, error] {
				return repoStore.SetDocFilePaths(ctx, missingURL, []string{"a"})
			},
		},
	}

	for _, tc := range errCases {
		res := tc.call()
		if res.IsOk() {
			t.Fatalf("%s unexpectedly succeeded for missing repository", tc.name)
		}
		classified, ok := errors.AsClassified(res.UnwrapErr())
		if !ok || classified.Category() != errors.CategoryNotFound {
			t.Fatalf("%s returned wrong error: %v", tc.name, res.UnwrapErr())
		}
	}
}

func testBuildOperations(t *testing.T) {
	store := createTestStore(t)
	ctx := t.Context()
	buildStore := store.Builds()

	// Create a build
	build := &Build{
		ID:          "build-123",
		Status:      BuildStatusRunning,
		StartTime:   time.Now(),
		TriggeredBy: "manual",
	}

	createResult := buildStore.Create(ctx, build)
	if createResult.IsErr() {
		t.Fatalf("Failed to create build: %v", createResult.UnwrapErr())
	}

	// Retrieve the build
	getResult := buildStore.GetByID(ctx, build.ID)
	if getResult.IsErr() {
		t.Fatalf("Failed to get build: %v", getResult.UnwrapErr())
	}

	if getResult.Unwrap().IsNone() {
		t.Fatal("Build not found after creation")
	}

	retrieved := getResult.Unwrap().Unwrap()
	if retrieved.Status != build.Status {
		t.Errorf("Expected status %v, got %v", build.Status, retrieved.Status)
	}

	// Update build status
	build.Status = BuildStatusCompleted
	build.EndTime = foundation.Some(time.Now())

	updateResult := buildStore.Update(ctx, build)
	if updateResult.IsErr() {
		t.Fatalf("Failed to update build: %v", updateResult.UnwrapErr())
	}

	// List builds
	listResult := buildStore.List(ctx, ListOptions{})
	if listResult.IsErr() {
		t.Fatalf("Failed to list builds: %v", listResult.UnwrapErr())
	}

	builds := listResult.Unwrap()
	if len(builds) != 1 {
		t.Errorf("Expected 1 build, got %d", len(builds))
	}
}

func testStatisticsOperations(t *testing.T) {
	store := createTestStore(t)
	ctx := t.Context()
	statsStore := store.Statistics()

	// Get initial statistics
	getResult := statsStore.Get(ctx)
	if getResult.IsErr() {
		t.Fatalf("Failed to get statistics: %v", getResult.UnwrapErr())
	}

	stats := getResult.Unwrap()
	initialBuilds := stats.TotalBuilds

	// Record a build
	build := &Build{
		ID:     "stats-test-build",
		Status: BuildStatusCompleted,
	}

	recordResult := statsStore.RecordBuild(ctx, build)
	if recordResult.IsErr() {
		t.Fatalf("Failed to record build: %v", recordResult.UnwrapErr())
	}

	// Verify statistics updated
	getResult = statsStore.Get(ctx)
	if getResult.IsErr() {
		t.Fatalf("Failed to get updated statistics: %v", getResult.UnwrapErr())
	}

	updatedStats := getResult.Unwrap()
	if updatedStats.TotalBuilds != initialBuilds+1 {
		t.Errorf("Expected total builds %d, got %d", initialBuilds+1, updatedStats.TotalBuilds)
	}
	if updatedStats.SuccessfulBuilds == 0 {
		t.Error("Expected successful builds to be incremented")
	}
}

func testTransactionOperations(t *testing.T) {
	t.Skip("FIXME: Deadlock in transaction test - needs refactoring of lock-free operations")

	store := createTestStore(t)
	ctx := t.Context()

	txResult := store.WithTransaction(ctx, func(txStore Store) error {
		// Create repository and build in transaction
		repo := &Repository{
			URL:    "https://github.com/tx/repo.git",
			Name:   "tx-repo",
			Branch: "main",
		}

		createResult := txStore.Repositories().Create(ctx, repo)
		if createResult.IsErr() {
			return createResult.UnwrapErr()
		}

		build := &Build{
			ID:          "tx-build",
			Status:      BuildStatusCompleted,
			StartTime:   time.Now(),
			TriggeredBy: "transaction-test",
		}

		buildResult := txStore.Builds().Create(ctx, build)
		if buildResult.IsErr() {
			return buildResult.UnwrapErr()
		}

		return nil
	})

	if txResult.IsErr() {
		t.Fatalf("Transaction failed: %v", txResult.UnwrapErr())
	}

	// Verify both items were created
	getRepoResult := store.Repositories().GetByURL(ctx, "https://github.com/tx/repo.git")
	if getRepoResult.IsErr() || getRepoResult.Unwrap().IsNone() {
		t.Error("Repository not found after transaction")
	}

	getBuildResult := store.Builds().GetByID(ctx, "tx-build")
	if getBuildResult.IsErr() || getBuildResult.Unwrap().IsNone() {
		t.Error("Build not found after transaction")
	}
}

func testHealthCheck(t *testing.T) {
	store := createTestStore(t)
	ctx := t.Context()

	healthResult := store.Health(ctx)
	if healthResult.IsErr() {
		t.Fatalf("Health check failed: %v", healthResult.UnwrapErr())
	}

	health := healthResult.Unwrap()
	if health.Status != healthyStatus {
		t.Errorf("Expected healthy status, got %q", health.Status)
	}
}

func testPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := t.Context()

	// Create initial store with data
	storeResult := NewJSONStore(tmpDir)
	if storeResult.IsErr() {
		t.Fatalf("Failed to create JSON store: %v", storeResult.UnwrapErr())
	}
	store := storeResult.Unwrap()

	// Add a repository to test persistence
	repo := &Repository{
		URL:    "https://github.com/test/persist.git",
		Name:   "persist-repo",
		Branch: "main",
	}
	if createResult := store.Repositories().Create(ctx, repo); createResult.IsErr() {
		t.Fatalf("Failed to create test repository: %v", createResult.UnwrapErr())
	}

	// Close current store
	closeResult := store.Close(ctx)
	if closeResult.IsErr() {
		t.Fatalf("Failed to close store: %v", closeResult.UnwrapErr())
	}

	// Create new store with same directory
	newStoreResult := NewJSONStore(tmpDir)
	if newStoreResult.IsErr() {
		t.Fatalf("Failed to create new store: %v", newStoreResult.UnwrapErr())
	}
	newStore := newStoreResult.Unwrap()

	// Verify data persisted
	listResult := newStore.Repositories().List(ctx)
	if listResult.IsErr() {
		t.Fatalf("Failed to list repositories from new store: %v", listResult.UnwrapErr())
	}

	repos := listResult.Unwrap()
	if len(repos) < 1 {
		t.Errorf("Expected at least 1 repository, got %d", len(repos))
	}

	// Check state file exists
	stateFile := filepath.Join(tmpDir, "daemon-state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file was not created")
	}
}

// createTestStore is a helper to create a test store.
func createTestStore(t *testing.T) Store {
	t.Helper()
	tmpDir := t.TempDir()
	storeResult := NewJSONStore(tmpDir)
	if storeResult.IsErr() {
		t.Fatalf("Failed to create JSON store: %v", storeResult.UnwrapErr())
	}
	return storeResult.Unwrap()
}

// TestStateService demonstrates the service integration.
func TestStateService(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create state service
	serviceResult := NewService(tmpDir)
	if serviceResult.IsErr() {
		t.Fatalf("Failed to create state service: %v", serviceResult.UnwrapErr())
	}
	service := serviceResult.Unwrap()

	ctx := t.Context()

	// Test service lifecycle
	t.Run("Service Lifecycle", func(t *testing.T) {
		// Start service
		if err := service.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}

		// Test health (Health() creates its own context internally)
		//nolint:contextcheck // Health() intentionally creates its own context for health checks
		health := service.Health()
		if health.Status != healthyStatus {
			t.Errorf("Expected healthy status, got %q with message: %s", health.Status, health.Message)
		}

		// Test dependencies
		deps := service.Dependencies()
		if len(deps) != 0 {
			t.Errorf("Expected no dependencies, got %v", deps)
		}

		// Stop service
		if err := service.Stop(ctx); err != nil {
			t.Fatalf("Failed to stop service: %v", err)
		}
	})

	// Test store access through service
	t.Run("Store Access", func(t *testing.T) {
		repoStore := service.GetRepositoryStore()
		buildStore := service.GetBuildStore()
		statsStore := service.GetStatisticsStore()

		if repoStore == nil {
			t.Error("Repository store is nil")
		}
		if buildStore == nil {
			t.Error("Build store is nil")
		}
		if statsStore == nil {
			t.Error("Statistics store is nil")
		}
	})

	// Test service statistics
	t.Run("Service Statistics", func(t *testing.T) {
		statsResult := service.GetStats(ctx)
		if statsResult.IsErr() {
			t.Fatalf("Failed to get service stats: %v", statsResult.UnwrapErr())
		}

		stats := statsResult.Unwrap()
		if stats.StoreType != "json" {
			t.Errorf("Expected store type 'json', got %q", stats.StoreType)
		}
	})
}

// Adapter tests removed after deprecation cleanup - use StateService directly instead
