package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The name of the file in a link is written the way a URL writes it:
// the space of the name is "%20". The fixer did not rewrite such a link,
// and the file it pointed at was renamed: the link was broken.
func TestTheLinkWithAnEncodedNameIsRewritten(t *testing.T) {
	dir := t.TempDir()

	page := filepath.Join(dir, "src.md")
	require.NoError(t, os.WriteFile(page, []byte("# Src\n\nSee [encoded](/Other%20Doc.md).\n"), 0o600))

	oldPath := filepath.Join(dir, "Other Doc.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("# Other\n"), 0o600))
	newPath := filepath.Join(dir, "other-doc.md")

	f := NewFixer(nil, false, false)
	links, err := f.findLinksInFile(page, oldPath)
	require.NoError(t, err)
	require.Len(t, links, 1)

	updates, err := f.applyLinkUpdates(links, oldPath, newPath)
	require.NoError(t, err)
	require.Len(t, updates, 1, "the link with the encoded name is rewritten")

	// #nosec G304 -- the path is the test's own file
	after, err := os.ReadFile(page)
	require.NoError(t, err)
	require.Contains(t, string(after), "[encoded](/other-doc.md)")
}
