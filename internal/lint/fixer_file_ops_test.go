package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsGitRepository tests git repository detection.
func TestIsGitRepository(t *testing.T) {
	t.Run("detects git repository", func(t *testing.T) {
		// Current directory should be a git repo
		isGit := isGitRepository(".")
		assert.True(t, isGit, "current directory should be a git repository")
	})

	t.Run("detects non-git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		isGit := isGitRepository(tmpDir)
		assert.False(t, isGit, "temp directory should not be a git repository")
	})
}

// TestBackupPreservesDirectoryStructure tests that backup preserves relative paths.
func TestBackupPreservesDirectoryStructure(t *testing.T) {
	// Create nested directory structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	apiDir := filepath.Join(docsDir, "api")
	err := os.MkdirAll(apiDir, 0o750)
	require.NoError(t, err)

	// Create files in different directories
	rootFile := filepath.Join(docsDir, "index.md")
	apiFile := filepath.Join(apiDir, "guide.md")

	err = os.WriteFile(rootFile, []byte("root content"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(apiFile, []byte("api content"), 0o600)
	require.NoError(t, err)

	// Create result
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: rootFile},
		},
		LinksUpdated: []LinkUpdate{
			{SourceFile: apiFile},
		},
	}

	// Create backup
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	backupDir, err := fixer.CreateBackup(result, docsDir)
	require.NoError(t, err)

	// Verify directory structure is preserved
	backupRoot := filepath.Join(backupDir, "index.md")
	backupAPI := filepath.Join(backupDir, "api", "guide.md")

	// #nosec G304 -- test utility reading backup files from test directory
	content, err := os.ReadFile(backupRoot)
	assert.NoError(t, err)
	assert.Equal(t, "root content", string(content))

	// #nosec G304 -- test utility reading backup files from test directory
	content, err = os.ReadFile(backupAPI)
	assert.NoError(t, err)
	assert.Equal(t, "api content", string(content))
}
