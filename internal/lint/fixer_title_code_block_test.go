package lint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The code of the document holds examples, not headings: the comment of
// a shell command is not the title of the page. The title drives the
// rendered heading and the entry in the sidebar.
func TestTheTitleIsNotTakenFromACodeBlock(t *testing.T) {
	body := []byte("## Install\n\n```bash\n# Install the tool with brew\nmake install\n```\n")

	title := extractTitleFromBody(body, "guide.md")
	require.NotEqual(t, "Install the tool with brew", title)
}

// The heading of the page is the title of the page.
func TestTheTitleIsTakenFromTheHeadingOfThePage(t *testing.T) {
	body := []byte("# The guide\n\nSome text.\n")

	require.Equal(t, "The guide", extractTitleFromBody(body, "guide.md"))
}
