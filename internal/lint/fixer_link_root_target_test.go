package lint

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A link that points at the root of the repository has no extension:
// "/Guide Setup". The name of the file has one: "Guide Setup.md". The
// link has to be rewritten, or it points at a page that does not exist
// any more.
func TestUpdateLinkTargetReplacesTheNameWithoutTheExtension(t *testing.T) {
	fixer := &Fixer{}

	link := LinkReference{
		SourceFile: filepath.Join("docs", "guide.md"),
		Target:     "/Guide Setup",
	}

	got := fixer.updateLinkTarget(link, "Guide Setup.md", "guide-setup.md")

	require.Equal(t, "/guide-setup", got)
}

// A link with the extension is rewritten as well.
func TestUpdateLinkTargetReplacesTheNameWithTheExtension(t *testing.T) {
	fixer := &Fixer{}

	link := LinkReference{
		SourceFile: filepath.Join("docs", "guide.md"),
		Target:     "Guide Setup.md",
	}

	got := fixer.updateLinkTarget(link, "Guide Setup.md", "guide-setup.md")

	require.Equal(t, "guide-setup.md", got)
}
