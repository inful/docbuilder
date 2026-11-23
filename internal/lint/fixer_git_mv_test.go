package lint

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The rename of a file of a repository moves it with git.
//
// The paths of git are relative to the directory of the -C flag, and
// the paths of the fixer are relative to the working directory of the
// process: the rename failed with "fatal: bad source" on every file of
// a repository, and the lint exited with an error.
func TestTheRenameOfARepositoryMovesTheFileWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	repo := t.TempDir()
	docs := filepath.Join(repo, "docs")
	require.NoError(t, os.MkdirAll(docs, 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(docs, "Aaa Doc.md"), []byte("# Doc\n"), 0o600))

	for _, args := range [][]string{{"init"}, {"add", "docs/Aaa Doc.md"}} {
		//nolint:gosec // the command is built from constants
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}

	// The fixer works with the paths of the walk of the linter: they
	// are relative to the working directory of the process.
	t.Chdir(repo)

	fixer := &Fixer{}
	require.True(t, fixer.shouldUseGitMv(filepath.Join("docs", "Aaa Doc.md")), "the file of the repository was not found")

	require.NoError(t, fixer.gitMv(filepath.Join("docs", "Aaa Doc.md"), filepath.Join("docs", "aaa-doc.md")))

	require.NoFileExists(t, filepath.Join("docs", "Aaa Doc.md"))
	require.FileExists(t, filepath.Join("docs", "aaa-doc.md"))

	// The rename is in the index of the repository.
	//nolint:gosec // the command is built from constants
	out, err := exec.CommandContext(t.Context(), "git", "-C", repo, "status", "--porcelain").CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(out), "docs/aaa-doc.md")
}
