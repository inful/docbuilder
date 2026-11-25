package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The walk of the rules and the walk of the broken links must use the
// same base for the excludes: the readme of the documentation is a page
// of the site, and its links were never checked.
func TestTheBrokenLinksOfTheReadmeOfTheDocumentationAreReported(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".git"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs", "sub"), 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "README.md"),
		[]byte("---\ntitle: Docs\n---\n\nSee [missing](./does-not-exist.md).\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "sub", "page.md"),
		[]byte("---\ntitle: Page\n---\n\nSee [missing](./does-not-exist.md).\n"), 0o600))

	broken, err := detectBrokenLinks(filepath.Join(repo, "docs"), nil)
	require.NoError(t, err)
	require.Len(t, broken, 2, "the broken link of the readme of the documentation was not reported")
}
