package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A page that shows the syntax of a link in its prose holds the
// example after the real link: the fixer rewrote the example and left
// the real link pointing at the old name.
func TestTheLinkInsideAnInlineCodeIsNotRewritten(t *testing.T) {
	dir := t.TempDir()

	oldPath := filepath.Join(dir, "Old-Name.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("# Old\n"), 0o600))

	page := filepath.Join(dir, "page.md")
	require.NoError(t, os.WriteFile(page, []byte(
		"# Page\n\nSee [the page](./Old-Name.md) and the syntax `](./Old-Name.md)` for reference.\n"), 0o600))

	f := NewFixer(nil, false, false)
	links, err := f.findLinksInFile(page, oldPath)
	require.NoError(t, err)
	require.Len(t, links, 1)

	updates, err := f.applyLinkUpdates(links, oldPath, filepath.Join(dir, "old-name.md"))
	require.NoError(t, err)
	require.Len(t, updates, 1)

	// #nosec G304 -- the path is the test's own file
	after, err := os.ReadFile(page)
	require.NoError(t, err)
	require.Contains(t, string(after), "[the page](./old-name.md)")
	require.Contains(t, string(after), "`](./Old-Name.md)`", "the example of the syntax was rewritten")
}
