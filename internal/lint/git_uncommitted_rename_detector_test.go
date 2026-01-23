package lint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitUncommittedRenameDetector_NotAGitRepo_ReturnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	detector := &GitUncommittedRenameDetector{}

	got, err := detector.DetectRenames(context.Background(), tmpDir)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGitUncommittedRenameDetector_DetectsStagedRename_GitMv(t *testing.T) {
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

	detector := &GitUncommittedRenameDetector{}
	got, err := detector.DetectRenames(ctx, repoDir)
	require.NoError(t, err)

	// Normalize to docs-root scope, to match the intended pipeline behavior.
	got, err = NormalizeRenameMappings(got, []string{docsDir})
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, oldPath, got[0].OldAbs)
	assert.Equal(t, newPath, got[0].NewAbs)
	assert.Equal(t, RenameSourceGitUncommitted, got[0].Source)
}

func TestGitUncommittedRenameDetector_DetectsUnstagedRename_FileMove(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	docsDir := filepath.Join(repoDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	oldPath := filepath.Join(docsDir, "old.md")
	newPath := filepath.Join(docsDir, "new.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("# Hello\n"), 0o600))

	git(t, repoDir, "add", "docs/old.md")
	git(t, repoDir, "commit", "-m", "add old")

	// Simulate a user rename outside of git mv (unstaged in index).
	require.NoError(t, os.Rename(oldPath, newPath))

	detector := &GitUncommittedRenameDetector{}
	got, err := detector.DetectRenames(ctx, repoDir)
	require.NoError(t, err)

	got, err = NormalizeRenameMappings(got, []string{docsDir})
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, oldPath, got[0].OldAbs)
	assert.Equal(t, newPath, got[0].NewAbs)
	assert.Equal(t, RenameSourceGitUncommitted, got[0].Source)
}

func TestGitUncommittedRenameDetector_IgnoresNonDocsRootRenames_AfterNormalization(t *testing.T) {
	ctx := context.Background()
	repoDir := initGitRepo(t)

	docsDir := filepath.Join(repoDir, "docs")
	otherDir := filepath.Join(repoDir, "other")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))
	require.NoError(t, os.MkdirAll(otherDir, 0o750))

	oldDoc := filepath.Join(otherDir, "old.md")
	require.NoError(t, os.WriteFile(oldDoc, []byte("# Hello\n"), 0o600))

	git(t, repoDir, "add", "other/old.md")
	git(t, repoDir, "commit", "-m", "add other")

	git(t, repoDir, "mv", "other/old.md", "other/new.md")

	detector := &GitUncommittedRenameDetector{}
	got, err := detector.DetectRenames(ctx, repoDir)
	require.NoError(t, err)

	got, err = NormalizeRenameMappings(got, []string{docsDir})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	git(t, repoDir, "init")
	git(t, repoDir, "config", "user.email", "test@example.com")
	git(t, repoDir, "config", "user.name", "Test")
	return repoDir
}

func git(t *testing.T, repoDir string, args ...string) {
	t.Helper()

	// #nosec G204 -- test helper executing git with controlled args
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", repoDir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
