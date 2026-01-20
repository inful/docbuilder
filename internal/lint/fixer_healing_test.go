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

func TestFixer_HealBrokenLinks(t *testing.T) {
	// Setup temporary directory for git repo
	tempDir := t.TempDir()

	// Init Git repo
	repo, err := git.PlainInit(tempDir, false)
	require.NoError(t, err)

	w, err := repo.Worktree()
	require.NoError(t, err)

	// 1. Create a file and commit it
	oldFile := filepath.Join(tempDir, "target.md")
	err = os.WriteFile(oldFile, []byte("# Target File\nContent"), 0o644)
	require.NoError(t, err)

	_, err = w.Add("target.md")
	require.NoError(t, err)

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)

	// 2. Rename the file manually and commit
	newFile := filepath.Join(tempDir, "moved_target.md")
	err = os.Rename(oldFile, newFile)
	require.NoError(t, err)

	_, err = w.Add("target.md") // Mark as deleted
	require.NoError(t, err)
	_, err = w.Add("moved_target.md") // Mark as added
	require.NoError(t, err)

	_, err = w.Commit("Move target file", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)

	// 3. Create a file with a broken link to the old path
	sourceFile := filepath.Join(tempDir, "index.md")
	err = os.WriteFile(sourceFile, []byte("# Index\n[Link](./target.md)"), 0o644)
	require.NoError(t, err)

	// 4. Initialize Fixer in the temp directory
	// We need to override the repo discovery for testing
	linter := NewLinter(nil)
	fixer := &Fixer{
		linter:   linter,
		gitRepo:  repo,
		gitAware: true,
		nowFn:    nil,
	}

	// 5. Run healing
	result := &FixResult{}
	fixer.healBrokenLinks(result, nil, tempDir)

	// 6. Verify link was updated
	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "[Link](moved_target.md)")
	assert.Len(t, result.LinksUpdated, 1)
	assert.Equal(t, "./target.md", result.LinksUpdated[0].OldTarget)
	assert.Equal(t, "moved_target.md", result.LinksUpdated[0].NewTarget)
}
