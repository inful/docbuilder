package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The fixer changes the categories of a page, and the report said
// nothing about it: the operator saw a changed sidebar and no reason
// for it.
func TestTheReportSaysWhatChangedInTheFrontmatter(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "x.md")
	require.NoError(t, os.WriteFile(page, []byte("---\ntitle: X\n---\n\n# X\n"), 0o600))

	fixer := NewFixer(NewLinter(&Config{}), false, false)
	result, err := fixer.Fix(dir)
	require.NoError(t, err)

	summary := result.Summary()
	require.True(t, strings.Contains(summary, "Frontmatter fields filled in"),
		"the report does not say which frontmatter fields were filled in:\n%s", summary)
}
