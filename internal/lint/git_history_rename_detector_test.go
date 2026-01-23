package lint

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHistoryRenameDetector_NotAGitRepo_ReturnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	detector := &GitHistoryRenameDetector{}

	got, err := detector.DetectRenames(context.Background(), tmpDir)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGitHistoryRenameDetector_UsesUpstreamRangeWhenAvailable(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	// Ensure we're on main to simplify upstream setup.
	git(t, repoDir, "checkout", "-b", "main")

	docsDir := filepath.Join(repoDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	oldPath := filepath.Join(docsDir, "old.md")
	newPath := filepath.Join(docsDir, "new.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("# Hello\n"), 0o600))

	git(t, repoDir, "add", "docs/old.md")
	git(t, repoDir, "commit", "-m", "add old")

	// Create a bare remote and push, setting upstream.
	remoteDir := t.TempDir()
	git(t, remoteDir, "init", "--bare")
	git(t, repoDir, "remote", "add", "origin", remoteDir)
	git(t, repoDir, "push", "-u", "origin", "main")

	// Now commit a rename locally (not pushed).
	git(t, repoDir, "mv", "docs/old.md", "docs/new.md")
	git(t, repoDir, "commit", "-m", "rename old to new")

	detector := &GitHistoryRenameDetector{}
	got, err := detector.DetectRenames(ctx, repoDir)
	require.NoError(t, err)

	got, err = NormalizeRenameMappings(got, []string{docsDir})
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, oldPath, got[0].OldAbs)
	assert.Equal(t, newPath, got[0].NewAbs)
	assert.Equal(t, RenameSourceGitHistory, got[0].Source)
}

func TestGitHistoryRenameDetector_FallbackWhenNoUpstream(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	docsDir := filepath.Join(repoDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	oldPath := filepath.Join(docsDir, "old.md")
	newPath := filepath.Join(docsDir, "new.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("# Hello\n"), 0o600))

	git(t, repoDir, "add", "docs/old.md")
	git(t, repoDir, "commit", "-m", "add old")

	git(t, repoDir, "mv", "docs/old.md", "docs/new.md")
	git(t, repoDir, "commit", "-m", "rename old to new")

	detector := &GitHistoryRenameDetector{}
	got, err := detector.DetectRenames(ctx, repoDir)
	require.NoError(t, err)

	got, err = NormalizeRenameMappings(got, []string{docsDir})
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, oldPath, got[0].OldAbs)
	assert.Equal(t, newPath, got[0].NewAbs)
	assert.Equal(t, RenameSourceGitHistory, got[0].Source)
}
