package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A link that starts with a slash points at the root of the repository
// of the documentation. The file is in the source tree, and the link
// must be found: a doc that follows the documented link style must not
// fail the lint.
func TestRepositoryRootRelativeLinkIsFound(t *testing.T) {
	docs := t.TempDir()

	howTo := filepath.Join(docs, "how-to")
	require.NoError(t, os.MkdirAll(howTo, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(howTo, "authentication.md"), []byte("# Authentication\n"), 0o600))

	index := filepath.Join(docs, "index.md")
	require.NoError(t, os.WriteFile(index, []byte("# Index\n\n[guide](/how-to/authentication.md)\n"), 0o600))

	resolved, err := resolveRelativePath(index, "/how-to/authentication.md")
	require.NoError(t, err)
	require.FileExists(t, resolved)
}

// A link that starts with a slash and points at nothing is still broken.
func TestRepositoryRootRelativeLinkToNothingIsBroken(t *testing.T) {
	docs := t.TempDir()
	index := filepath.Join(docs, "index.md")
	require.NoError(t, os.WriteFile(index, []byte("# Index\n"), 0o600))

	resolved, err := resolveRelativePath(index, "/how-to/nothing.md")
	require.NoError(t, err)
	require.NoFileExists(t, resolved)
}
