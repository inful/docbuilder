package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The excludes hold for a single file too: the pre-commit hook lints
// every staged file on its own, and the readme of the repository is
// never reported. The hook blocked the commit of every readme.
func TestTheSingleFileLintHonoursTheExcludes(t *testing.T) {
	repo := t.TempDir()
	// A repository: the excludes are paths of the repository.
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".git"), 0o750))

	readme := filepath.Join(repo, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("# The project\n"), 0o600))

	result, err := NewLinter(&Config{}).LintPath(readme)
	require.NoError(t, err)
	require.Empty(t, result.Issues, "the readme of the repository is excluded")
	require.Equal(t, 0, result.FilesTotal, "the excluded file is not linted")
}

// A file that is not excluded is linted as before.
func TestTheSingleFileLintLintsTheFilesThatAreNotExcluded(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".git"), 0o750))

	page := filepath.Join(repo, "guide.md")
	require.NoError(t, os.WriteFile(page, []byte("# Guide\n"), 0o600))

	result, err := NewLinter(&Config{}).LintPath(page)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues, "the page without frontmatter is reported")
	require.Equal(t, 1, result.FilesTotal)
}
