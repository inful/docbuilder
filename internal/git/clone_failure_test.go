package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A clone that fails leaves the working copy of the repository alone:
// the old copy was removed before the clone, and the pages of the
// repository were gone until a later clone succeeded.
func TestAFailedCloneLeavesTheOldCopyAlone(t *testing.T) {
	workspace := t.TempDir()

	// The repository that is already there.
	repoPath := filepath.Join(workspace, "repo")
	require.NoError(t, os.MkdirAll(repoPath, 0o750))
	marker := filepath.Join(repoPath, "docs", "guide.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(marker), 0o750))
	require.NoError(t, os.WriteFile(marker, []byte("# Guide\n"), 0o600))

	client := NewClient(workspace)

	// The clone fails: the address does not exist.
	_, err := client.CloneRepo(t.Context(), config.Repository{
		Name: "repo",
		URL:  filepath.Join(workspace, "does-not-exist"),
	})
	require.Error(t, err, "the clone of a repository that does not exist succeeded")

	// The old copy is there.
	_, err = os.Stat(marker)
	require.NoError(t, err, "the working copy of the repository was taken by the failed clone")
}
