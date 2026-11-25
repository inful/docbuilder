package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The usage of a reference-style link whose label is not defined is
// left as plain text by the parser: the page shows the text of the link
// and the navigation is dead. Nothing reported it.
func TestTheUsageOfAnUndefinedReferenceIsReported(t *testing.T) {
	dir := t.TempDir()

	page := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(page, []byte("---\ntitle: A\n---\n\n# A\n\nSee [the missing page][nope].\n"), 0o600))

	broken, err := detectBrokenLinks(dir, nil)
	require.NoError(t, err)
	require.Len(t, broken, 1, "the usage of the undefined reference is broken")
	require.Equal(t, "nope", broken[0].Target)
	require.Equal(t, 7, broken[0].LineNumber, "the line of the usage is reported")
}

// The usage of a reference-style link whose label is defined is a link
// to the file of the definition.
func TestTheUsageOfADefinedReferenceIsNotBroken(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "known.md"), []byte("---\ntitle: Known\n---\n\n# Known\n"), 0o600))
	page := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(page, []byte("---\ntitle: A\n---\n\n# A\n\nSee [the known page][known].\n\n[known]: ./known.md\n"), 0o600))

	broken, err := detectBrokenLinks(dir, nil)
	require.NoError(t, err)
	require.Empty(t, broken, "the usage of the defined reference points at a file that exists")
}

// The examples of the inline code are not links: the syntax of a
// reference-style link is shown in the documentation of the linter.
func TestTheUsageInAnInlineCodeIsNotReported(t *testing.T) {
	dir := t.TempDir()

	page := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(page, []byte("---\ntitle: A\n---\n\n# A\n\nThe syntax of a link is `[text][ref]`.\n"), 0o600))

	broken, err := detectBrokenLinks(dir, nil)
	require.NoError(t, err)
	require.Empty(t, broken, "the example of the inline code is not a link")
}
