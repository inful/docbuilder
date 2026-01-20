package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	helpers "git.home.luguber.info/inful/docbuilder/internal/testutil/testutils"
)

func TestFixer_Rollback(t *testing.T) {
	// Setup temporary directory and git repo
	repo, w, tempDir := helpers.SetupTestGitRepo(t)

	// 1. Create a file and commit it (Initial State)
	file1 := filepath.Join(tempDir, "file1.md")
	err := os.WriteFile(file1, []byte("Content 1"), 0o600)
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
	// Modify existing file
	err = os.WriteFile(file1, []byte("Modified Content"), 0o600)
	require.NoError(t, err)
	_, err = w.Add("file1.md")
	require.NoError(t, err)

	// Create new file
	newFile := filepath.Join(tempDir, "file2.md")
	err = os.WriteFile(newFile, []byte("New File"), 0o600)
	require.NoError(t, err)
	_, err = w.Add("file2.md")
	require.NoError(t, err)

	// Simulate a rename (staged)
	oldPathInRepo := "file1.md"
	newPathInRepo := "file1_new.md"
	err = os.Rename(file1, filepath.Join(tempDir, newPathInRepo))
	require.NoError(t, err)
	_, err = w.Remove(oldPathInRepo)
	require.NoError(t, err)
	_, err = w.Add(newPathInRepo)
	require.NoError(t, err)

	// 4. Perform rollback
	err = fixer.rollback()
	require.NoError(t, err)

	// 5. Verify state
	// Original file should be restored
	// #nosec G304 -- test file
	content, err := os.ReadFile(file1)
	require.NoError(t, err)
	assert.Equal(t, "Content 1", string(content))

	// New file should be gone
	_, err = os.Stat(newFile)
	assert.True(t, os.IsNotExist(err), "file2.md should have been removed by hard reset")

	// Renamed file should be gone
	_, err = os.Stat(filepath.Join(tempDir, newPathInRepo))
	assert.True(t, os.IsNotExist(err), "file1_new.md should have been removed by hard reset")
}
