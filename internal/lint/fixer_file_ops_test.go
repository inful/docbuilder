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
		// Pre-flight: the function under test walks up the directory
		// tree looking for a .git. If t.TempDir() somehow lands inside a
		// git working tree (e.g. because a prior test in the same
		// process left a .git in an ancestor of the system temp dir,
		// or because a subprocess of a sibling test polluted TMPDIR),
		// the negative assertion is meaningless — the test would be
		// exercising the function against an invalid input, not the
		// contract it intends to test. Skip in that case with a
		// descriptive message rather than failing on environmental
		// noise outside the function's responsibility.
		if isInsideAnyGitTree(t, tmpDir) {
			t.Skipf("t.TempDir() landed inside a git working tree (parent has .git); environment is dirty, cannot exercise negative case")
		}
		isGit := isGitRepository(tmpDir)
		assert.False(t, isGit, "temp directory should not be a git repository")
	})
}

// isInsideAnyGitTree returns true if any ancestor directory of dir (or
// dir itself) contains a .git entry. Used by TestIsGitRepository to
// guard the negative case against environmental noise.
func isInsideAnyGitTree(t *testing.T, dir string) bool {
	t.Helper()
	cur, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return false
		}
		cur = parent
	}
}
