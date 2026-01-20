package helpers

import (
	"testing"

	"github.com/go-git/go-git/v5"
)

// SetupTestGitRepo initializes a temporary git repository for testing.
// Returns the repository, its worktree, and the absolute path to the temporary directory.
func SetupTestGitRepo(t *testing.T) (*git.Repository, *git.Worktree, string) {
	t.Helper()

	tempDir := t.TempDir()

	repo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("failed to initialize git repo: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	return repo, w, tempDir
}
