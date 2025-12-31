package state

import (
	"testing"
	"time"
)

const testRepoURL = "https://github.com/test/repo"

func TestServiceAdapter(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create the underlying service
	serviceResult := NewService(tmpDir)
	if serviceResult.IsErr() {
		t.Fatalf("Failed to create service: %v", serviceResult.UnwrapErr())
	}
	service := serviceResult.Unwrap()

	// Create the adapter
	adapter := NewServiceAdapter(service)

	t.Run("LifecycleManager interface", func(t *testing.T) {
		// Test Load
		if err := adapter.Load(); err != nil {
			t.Errorf("Load() failed: %v", err)
		}

		// Test IsLoaded
		if !adapter.IsLoaded() {
			t.Error("IsLoaded() should return true after Load()")
		}

		// Test Save
		if err := adapter.Save(); err != nil {
			t.Errorf("Save() failed: %v", err)
		}

		// Test LastSaved
		if adapter.LastSaved() == nil {
			t.Error("LastSaved() should return non-nil after Save()")
		}
	})

	t.Run("RepositoryInitializer interface", func(t *testing.T) {
		url := testRepoURL
		name := "test-repo"
		branch := "main"

		// Ensure repository state creates a new entry
		adapter.EnsureRepositoryState(url, name, branch)

		// Verify the repository was created
		docHash := adapter.GetRepoDocFilesHash(url)
		// Should return empty string for new repo
		if docHash != "" {
			t.Errorf("Expected empty doc hash for new repo, got: %s", docHash)
		}
	})

	t.Run("RepositoryMetadataWriter interface", func(t *testing.T) {
		url := testRepoURL

		// Set document count
		adapter.SetRepoDocumentCount(url, 42)

		// Set doc files hash
		adapter.SetRepoDocFilesHash(url, "abc123hash")

		// Verify the hash was set
		hash := adapter.GetRepoDocFilesHash(url)
		if hash != "abc123hash" {
			t.Errorf("Expected hash 'abc123hash', got: %s", hash)
		}
	})

	t.Run("RepositoryMetadataStore interface", func(t *testing.T) {
		url := testRepoURL
		paths := []string{"docs/index.md", "docs/guide.md", "docs/api.md"}

		// Set doc file paths
		adapter.SetRepoDocFilePaths(url, paths)

		// Get doc file paths
		gotPaths := adapter.GetRepoDocFilePaths(url)
		if len(gotPaths) != len(paths) {
			t.Errorf("Expected %d paths, got %d", len(paths), len(gotPaths))
		}
		for i, p := range paths {
			if gotPaths[i] != p {
				t.Errorf("Path mismatch at index %d: expected %s, got %s", i, p, gotPaths[i])
			}
		}
	})

	t.Run("RepositoryCommitTracker interface", func(t *testing.T) {
		url := testRepoURL
		commit := "abc123def456"

		// Set last commit
		adapter.SetRepoLastCommit(url, "test-repo", "main", commit)

		// Get last commit
		gotCommit := adapter.GetRepoLastCommit(url)
		if gotCommit != commit {
			t.Errorf("Expected commit '%s', got: %s", commit, gotCommit)
		}
	})

	t.Run("RepositoryBuildCounter interface", func(t *testing.T) {
		url := testRepoURL

		// Increment build count (success)
		adapter.IncrementRepoBuild(url, true)

		// Increment build count (failure)
		adapter.IncrementRepoBuild(url, false)

		// No direct getter in narrow interface, but the operation should not panic
	})

	t.Run("ConfigurationStateStore interface", func(t *testing.T) {
		// Test config hash
		adapter.SetLastConfigHash("config-hash-123")
		if got := adapter.GetLastConfigHash(); got != "config-hash-123" {
			t.Errorf("Expected config hash 'config-hash-123', got: %s", got)
		}

		// Test report checksum
		adapter.SetLastReportChecksum("report-checksum-456")
		if got := adapter.GetLastReportChecksum(); got != "report-checksum-456" {
			t.Errorf("Expected report checksum 'report-checksum-456', got: %s", got)
		}

		// Test global doc files hash
		adapter.SetLastGlobalDocFilesHash("global-hash-789")
		if got := adapter.GetLastGlobalDocFilesHash(); got != "global-hash-789" {
			t.Errorf("Expected global hash 'global-hash-789', got: %s", got)
		}
	})

	t.Run("RecordDiscovery method", func(t *testing.T) {
		url := "https://github.com/test/another-repo"

		// First ensure the repo exists
		adapter.EnsureRepositoryState(url, "another-repo", "main")

		// Record discovery
		adapter.RecordDiscovery(url, 15)

		// The operation should not panic and should update the repository
		// We can't easily verify statistics without the full interface, but
		// the operation completing without error is the main check
	})

	t.Run("DaemonStateManager compile-time verification", func(t *testing.T) {
		// This test verifies at compile time that ServiceAdapter implements DaemonStateManager
		var _ DaemonStateManager = adapter
	})

	t.Run("Empty URL handling", func(t *testing.T) {
		// These should not panic and should be no-ops
		adapter.EnsureRepositoryState("", "name", "branch")
		adapter.SetRepoDocumentCount("", 10)
		adapter.SetRepoDocFilesHash("", "hash")
		adapter.SetRepoDocFilePaths("", []string{"path"})
		adapter.SetRepoLastCommit("", "name", "branch", "commit")
		adapter.IncrementRepoBuild("", true)
		adapter.RecordDiscovery("", 10)

		// Getters should return empty/nil for empty URL
		if got := adapter.GetRepoDocFilesHash(""); got != "" {
			t.Errorf("Expected empty string for empty URL, got: %s", got)
		}
		if got := adapter.GetRepoDocFilePaths(""); got != nil {
			t.Errorf("Expected nil for empty URL, got: %v", got)
		}
		if got := adapter.GetRepoLastCommit(""); got != "" {
			t.Errorf("Expected empty string for empty URL, got: %s", got)
		}
	})

	t.Run("Non-existent repository handling", func(t *testing.T) {
		nonExistentURL := "https://github.com/does/not-exist"

		// Getters should return empty/nil for non-existent repos
		if got := adapter.GetRepoDocFilesHash(nonExistentURL); got != "" {
			t.Errorf("Expected empty string for non-existent repo, got: %s", got)
		}
		if got := adapter.GetRepoDocFilePaths(nonExistentURL); got != nil {
			t.Errorf("Expected nil for non-existent repo, got: %v", got)
		}
		if got := adapter.GetRepoLastCommit(nonExistentURL); got != "" {
			t.Errorf("Expected empty string for non-existent repo, got: %s", got)
		}
	})
}

func TestServiceAdapterSaveTimestamp(t *testing.T) {
	tmpDir := t.TempDir()

	serviceResult := NewService(tmpDir)
	if serviceResult.IsErr() {
		t.Fatalf("Failed to create service: %v", serviceResult.UnwrapErr())
	}

	adapter := NewServiceAdapter(serviceResult.Unwrap())

	// Initially LastSaved should be nil
	if adapter.LastSaved() != nil {
		t.Error("LastSaved() should be nil initially")
	}

	// After Save, it should be set
	before := time.Now()
	if err := adapter.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}
	after := time.Now()

	saved := adapter.LastSaved()
	if saved == nil {
		t.Fatal("LastSaved() should not be nil after Save()")
	}
	if saved.Before(before) || saved.After(after) {
		t.Errorf("LastSaved() timestamp out of expected range")
	}
}
