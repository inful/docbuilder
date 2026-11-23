package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The fixer looks for the links to every renamed file, and the walk of
// the tree was repeated for each of them. The list of the files is
// collected once.
func TestMarkdownFilesAreCollectedOnce(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("# Page\n"), 0o600))
	}

	fixer := &Fixer{}

	first, err := fixer.markdownFilesOf(dir)
	require.NoError(t, err)
	require.Len(t, first, 3)

	second, err := fixer.markdownFilesOf(dir)
	require.NoError(t, err)
	require.Len(t, second, 3)
	require.Equal(t, &first[0], &second[0], "the list of the files is not the one that was collected")
}
