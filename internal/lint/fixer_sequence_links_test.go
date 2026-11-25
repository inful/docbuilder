package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The fixer renames the files of a sequence directory, and the links
// inside them have to be rewritten as well: the list of the files of
// the tree held the old name, and the file was read from a path that
// does not exist any more.
func TestTheSequenceRenamerRewritesTheLinksOfTheRenamedFile(t *testing.T) {
	dir := t.TempDir()
	adrDir := filepath.Join(dir, "adr")
	require.NoError(t, os.MkdirAll(adrDir, 0o750))

	// Two files of the sequence: the first one links to the second.
	require.NoError(t, os.WriteFile(filepath.Join(adrDir, "a.md"), []byte("# A\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(adrDir, "m-link.md"), []byte("# Link\n\nSee [y](y.md).\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(adrDir, "y.md"), []byte("# Y\n"), 0o600))

	fixer := NewFixer(NewLinter(&Config{}), false, false)
	result, err := fixer.Fix(dir)
	require.NoError(t, err)
	require.Empty(t, result.Errors, "the fixer reported errors")

	// The files were renamed.
	require.FileExists(t, filepath.Join(adrDir, "adr-002-m-link.md"))
	require.FileExists(t, filepath.Join(adrDir, "adr-003-y.md"))

	// The link inside the renamed file points at the renamed file.
	// #nosec G304 -- the path is the file of the test.
	content, err := os.ReadFile(filepath.Join(adrDir, "adr-002-m-link.md"))
	require.NoError(t, err)
	require.Contains(t, string(content), "adr-003-y.md",
		"the link inside the renamed file was not rewritten")
}
