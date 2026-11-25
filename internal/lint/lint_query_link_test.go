package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The query of a link is not part of the file: the link points at the
// file, and the query is what the reader sends to it. The linter
// reported the link as broken and blocked the build.
func TestALinkWithAQueryIsNotABrokenLink(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "exists.md"), []byte("---\ntitle: Exists\n---\n\n# Exists\n"), 0o600))
	page := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(page, []byte("---\ntitle: A\n---\n\n- query link: [E](./exists.md?ref=foo)\n"), 0o600))

	broken, err := detectBrokenLinks(dir, nil)
	require.NoError(t, err)
	require.Empty(t, broken, "the link with a query points at a file that exists")
}
