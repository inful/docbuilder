package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Two links with the same target can be on the same line. The fixer
// rewrote only the last one, and the first one pointed at a file that
// does not exist any more: the fixer produced a broken link.
func TestBothLinksWithTheSameTargetOnOneLineAreRewritten(t *testing.T) {
	dir := t.TempDir()

	page := filepath.Join(dir, "src.md")
	content := "# Src\n\nSee [a](./Other Doc.md) and [b](./Other Doc.md).\n"
	require.NoError(t, os.WriteFile(page, []byte(content), 0o600))

	oldPath := filepath.Join(dir, "Other Doc.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("# Other\n"), 0o600))
	newPath := filepath.Join(dir, "other-doc.md")

	f := NewFixer(nil, false, false)
	links, err := f.findLinksInFile(page, oldPath)
	require.NoError(t, err)
	require.Len(t, links, 2)
	require.Equal(t, links[0].LineNumber, links[1].LineNumber, "the two links are on the same line")

	updates, err := f.applyLinkUpdates(links, oldPath, newPath)
	require.NoError(t, err)
	require.Len(t, updates, 2, "both links are rewritten")

	// #nosec G304 -- the path is the test's own file
	after, err := os.ReadFile(page)
	require.NoError(t, err)
	require.Contains(t, string(after), "[a](./other-doc.md)")
	require.Contains(t, string(after), "[b](./other-doc.md)")
}
