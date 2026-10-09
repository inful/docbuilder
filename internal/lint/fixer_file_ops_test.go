package lint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsGitRepository tests git repository detection. Each subtest builds
// its own setup in t.TempDir() so the result does not depend on the
// runner's CWD or on the presence of a .git directory at any specific
// path on the host.
func TestIsGitRepository(t *testing.T) {
	gitInit := func(t *testing.T, dir string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// #nosec G204 -- dir is t.TempDir()-scoped
		init := exec.CommandContext(ctx, "git", "init", "--initial-branch=main", dir)
		init.Stdout = os.Stderr
		init.Stderr = os.Stderr
		require.NoError(t, init.Run(), "git init in temp dir should succeed")
	}

	t.Run("detects git repository", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)

		isGit := isGitRepository(repoDir)
		assert.True(t, isGit, "directory with a freshly-initialized git repo should be detected")
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
	linter := NewLinter(&Config{Format: formatText})
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
