package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The fixer rewrites the target of a link and not the prose of the page.
// A file that is mentioned in the prose and linked to on the same page
// used to have its prose rewritten and its link left broken.
func TestLinkUpdateRewritesTheLinkAndNotTheProse(t *testing.T) {
	dir := t.TempDir()

	guide := filepath.Join(dir, "guide.md")
	content := "# Guide\n\nCopy Deploy.md into the image before you build.\n\nSee the [steps](Deploy.md) for details.\n"
	require.NoError(t, os.WriteFile(guide, []byte(content), 0o600))

	fixer := NewFixer(nil, false, false)
	updates, err := fixer.applyLinkUpdates([]LinkReference{{
		SourceFile: guide,
		LineNumber: 5,
		Target:     "Deploy.md",
	}}, "Deploy.md", "deploy.md")
	require.NoError(t, err)
	require.Len(t, updates, 1)

	// #nosec G304 -- the path is the test's own file
	after, err := os.ReadFile(guide)
	require.NoError(t, err)

	// The prose keeps its text, the link points at the new name.
	require.Contains(t, string(after), "Copy Deploy.md into the image")
	require.Contains(t, string(after), "[steps](deploy.md)")
}

// The line of a link is the line of the link, and not the line of a
// sentence that mentions the same file.
func TestLinkLineIsTheLineOfTheLink(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "guide.md")
	content := "# Guide\n\nCopy Deploy.md into the image.\n\nSee the [steps](Deploy.md) for details.\n"
	require.NoError(t, os.WriteFile(page, []byte(content), 0o600))

	target := filepath.Join(dir, "Deploy.md")
	require.NoError(t, os.WriteFile(target, []byte("# Deploy\n"), 0o600))

	f := NewFixer(nil, false, false)
	links, err := f.findLinksInFile(page, target)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, 5, links[0].LineNumber)
}
