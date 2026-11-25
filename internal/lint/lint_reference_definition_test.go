package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A definition of a reference and the usage of it point at the same
// file: the broken link was reported twice.
func TestTheDefinitionOfAReferenceIsNotReportedTwice(t *testing.T) {
	dir := t.TempDir()

	page := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(page, []byte("---\ntitle: A\n---\n\n# A\n\nSee [the broken page][nope].\n\n[nope]: ./nope.md\n"), 0o600))

	broken, err := detectBrokenLinks(dir, nil)
	require.NoError(t, err)
	require.Len(t, broken, 1, "the broken link is reported once")
	require.Equal(t, "./nope.md", broken[0].Target)
}
