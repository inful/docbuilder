package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The list of the files of the tree follows a rename.
//
// The list is collected once per run: the fixer looks for the links to
// every file it renames, and the walk of the tree was repeated for
// each of them. After a rename the list held a path that does not
// exist any more, the scan of it failed, and the links inside the
// renamed file were never rewritten.
func TestTheListOfTheFilesFollowsARename(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.md")
	newPath := filepath.Join(dir, "new.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("# Page\n"), 0o600))

	fixer := &Fixer{}
	files, err := fixer.markdownFilesOf(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	require.NoError(t, os.Rename(oldPath, newPath))
	fixer.replaceScannedFile(oldPath, newPath)

	files, err = fixer.markdownFilesOf(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, newPath, files[0])

	// The file of the list can be read.
	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	require.NotEmpty(t, content)
}
