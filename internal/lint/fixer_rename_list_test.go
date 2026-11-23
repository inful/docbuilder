package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The list of the files of the tree follows a rename, whatever the
// shape of the paths.
//
// The files of the tree and the files of the issues do not have the
// same shape: one of them is relative to the working directory of the
// process and the other one is absolute. The comparison of the two
// never matched, and the links inside the renamed file were never
// rewritten.
func TestTheListOfTheFilesFollowsARenameWithARelativeRoot(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docs, 0o750))

	oldPath := filepath.Join(docs, "A Page.md")
	newPath := filepath.Join(docs, "a-page.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("# Page\n"), 0o600))

	// The lint runs with a relative root.
	t.Chdir(dir)

	fixer := &Fixer{}
	files, err := fixer.markdownFilesOf("docs")
	require.NoError(t, err)
	require.Len(t, files, 1)

	// The fixer renames the file and then puts the new name in the
	// list.
	require.NoError(t, os.Rename(oldPath, newPath))
	fixer.replaceScannedFile(filepath.Join("docs", "A Page.md"), filepath.Join("docs", "a-page.md"))

	files, err = fixer.markdownFilesOf("docs")
	require.NoError(t, err)
	require.Len(t, files, 1)

	// The file of the list is the new one, and it can be read.
	require.FileExists(t, files[0])
	content, err := os.ReadFile(files[0]) // #nosec G304 -- the path is the test's own
	require.NoError(t, err)
	require.NotEmpty(t, content)
}

// A relative path and an absolute path are the same path.
func TestARelativePathAndAnAbsolutePathAreTheSamePath(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "docs", "page.md")

	t.Chdir(dir)

	require.True(t, pathsEqualCaseInsensitive(abs, filepath.Join("docs", "page.md")))
	require.False(t, pathsEqualCaseInsensitive(abs, filepath.Join("docs", "other.md")))
}
