package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixer_Rollback(t *testing.T) {
	// Setup temporary directory for git repo
	tempDir := t.TempDir()

	// Init Git repo
	repo, err := git.PlainInit(tempDir, false)
	require.NoError(t, err)

	w, err := repo.Worktree()
	require.NoError(t, err)

	// 1. Create a file and commit it (Initial State)
	file1 := filepath.Join(tempDir, "file1.md")
	err = os.WriteFile(file1, []byte("Content 1"), 0o644)
	require.NoError(t, err)

	_, err = w.Add("file1.md")
	require.NoError(t, err)

	headSHA, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)

	// 2. Initialize Fixer
	fixer := &Fixer{
		gitRepo:    repo,
		initialSHA: headSHA,
		gitAware:   true,
	}

	// 3. Make some "fixes"
	err = os.WriteFile(file1, []byte("Modified Content"), 0o644)
	require.NoError(t, err)

	newFile := filepath.Join(tempDir, "file2.md")
	err = os.WriteFile(newFile, []byte("New File"), 0o644)
	require.NoError(t, err)

	// 4. Perform rollback
	err = fixer.rollback()
	require.NoError(t, err)

	// 5. Verify state
	// #nosec G304 -- test file
	content, err := os.ReadFile(file1)
	require.NoError(t, err)
	assert.Equal(t, "Content 1", string(content))

	_, err = os.Stat(newFile)
	assert.Error(t, err, "file2.md should have been removed by hard reset")
}
