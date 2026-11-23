package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A link to the readme of the repository is broken.
//
// The readme of the repository is above the documentation, and the site
// has no page for it: the walk that looks for the file stopped at the
// root of the repository, and the link was judged valid.
func TestALinkToTheReadmeOfTheRepositoryIsBroken(t *testing.T) {
	repo := t.TempDir()
	docs := filepath.Join(repo, "docs")
	require.NoError(t, os.MkdirAll(docs, 0o750))

	// The readme of the repository is above the documentation.
	require.NoError(t, os.WriteFile(filepath.Join(repo, "README.md"), []byte("# The repository\n"), 0o600))

	page := filepath.Join(docs, "index.md")
	require.NoError(t, os.WriteFile(page, []byte("# Index\n\n[readme](/README.md)\n"), 0o600))

	broken, err := detectBrokenLinksInFile(page, docs)
	require.NoError(t, err)
	require.Len(t, broken, 1, "the link to the readme of the repository is not broken")
}

// A link to a file of the documentation is found.
func TestALinkToAFileOfTheDocumentationIsFound(t *testing.T) {
	docs := t.TempDir()
	howTo := filepath.Join(docs, "how-to")
	require.NoError(t, os.MkdirAll(howTo, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(howTo, "authentication.md"), []byte("# Authentication\n"), 0o600))

	page := filepath.Join(docs, "index.md")
	require.NoError(t, os.WriteFile(page, []byte("# Index\n\n[guide](/how-to/authentication.md)\n"), 0o600))

	broken, err := detectBrokenLinksInFile(page, docs)
	require.NoError(t, err)
	require.Empty(t, broken)
}
