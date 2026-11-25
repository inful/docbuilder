package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The link "/bar" points at a file of the site, and the root of the
// site is the root of the repository. The lint of a single file used
// the file as the root of the tree, and the walk that resolves the
// link stopped at once: the link was reported as broken. The
// pre-commit hook lints every staged file on its own, and the commit
// was blocked.
func TestTheSingleFileLintResolvesTheLinksOfTheSite(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".git"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs", "sub"), 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "bar.md"), []byte("---\ntitle: Bar\n---\n\n# Bar\n"), 0o600))
	page := filepath.Join(repo, "docs", "sub", "foo.md")
	require.NoError(t, os.WriteFile(page, []byte("---\ntitle: Foo\n---\n\nSee [bar](/bar).\n"), 0o600))

	result, err := NewLinter(&Config{}).LintPath(page)
	require.NoError(t, err)

	for _, issue := range result.Issues {
		if issue.Rule == "broken-links" {
			t.Fatalf("the link to the page of the site was reported as broken: %s", issue.Message)
		}
	}
}
