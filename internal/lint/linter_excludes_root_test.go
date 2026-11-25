package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The excludes are paths of the repository: the same file was linted or
// not depending on the directory the linter was started in. The readme
// of the documentation is a page of the site, and the readme of the
// repository is not.
func TestTheExcludesAreThePathsOfTheRepository(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".git"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs"), 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(repo, "README.md"), []byte("# The repository\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "README.md"), []byte("# The documentation\n"), 0o600))

	docs, err := NewLinter(&Config{}).LintPath(filepath.Join(repo, "docs"))
	require.NoError(t, err)
	require.Equal(t, 1, docs.FilesTotal, "the readme of the documentation is a page of the site")

	all, err := NewLinter(&Config{}).LintPath(repo)
	require.NoError(t, err)
	require.Equal(t, 1, all.FilesTotal, "the readme of the repository is excluded")
}
