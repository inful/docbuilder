package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixer_HealsBrokenLinks_FromGitUncommittedRename(t *testing.T) {
	repoDir := initGitRepo(t)
	docsDir := filepath.Join(repoDir, "docs")
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "old"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "new"), 0o750))

	oldTarget := filepath.Join(docsDir, "old", "target.md")
	indexFile := filepath.Join(docsDir, "index.md")

	require.NoError(t, os.WriteFile(oldTarget, []byte("# Target\n"), 0o600))
	require.NoError(t, os.WriteFile(indexFile, []byte("[Go](old/target.md)\n"), 0o600))

	git(t, repoDir, "add", "docs/old/target.md", "docs/index.md")
	git(t, repoDir, "commit", "-m", "add docs")

	// User moves the file (staged rename) and forgets to update links.
	git(t, repoDir, "mv", "docs/old/target.md", "docs/new/target.md")

	// Sanity: link is currently broken.
	before, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)
	require.Len(t, before, 1)

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, true)
	res, err := fixer.fix(docsDir)
	require.NoError(t, err)

	// The broken link should be healed and no broken links should remain.
	require.Empty(t, res.BrokenLinks)

	// The index link should now point at the new location.
	// #nosec G304 -- test reads from a tempdir path
	data, err := os.ReadFile(indexFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "[Go](new/target.md)")

	// Ensure the update is recorded.
	require.NotEmpty(t, res.LinksUpdated)
}
