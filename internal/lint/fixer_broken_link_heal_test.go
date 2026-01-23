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

func TestFixer_SkipsBrokenLinkHealing_WhenRenameMappingIsAmbiguous(t *testing.T) {
	repoDir := initGitRepo(t)
	docsDir := filepath.Join(repoDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	oldFoo := filepath.Join(docsDir, "Foo.md")
	oldfoo := filepath.Join(docsDir, "foo.md")
	indexFile := filepath.Join(docsDir, "index.md")

	require.NoError(t, os.WriteFile(oldFoo, []byte("# Foo\n"), 0o600))
	require.NoError(t, os.WriteFile(oldfoo, []byte("# foo\n"), 0o600))
	// Use a case-mismatched link target so it won't match the exact-case mapping,
	// forcing the healer into its case-insensitive matching path.
	require.NoError(t, os.WriteFile(indexFile, []byte("[Foo](FOO.md)\n"), 0o600))

	git(t, repoDir, "add", "docs/Foo.md", "docs/foo.md", "docs/index.md")
	git(t, repoDir, "commit", "-m", "add docs")

	// User renames both files (staged renames) and forgets to update links.
	git(t, repoDir, "mv", "docs/Foo.md", "docs/FooNew.md")
	git(t, repoDir, "mv", "docs/foo.md", "docs/fooNew.md")

	// Sanity: link is currently broken.
	before, err := detectBrokenLinks(indexFile)
	require.NoError(t, err)
	require.Len(t, before, 1)

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, true)
	// Fix only the linking file so filename-convention renames on other files
	// cannot affect ambiguity detection.
	res, err := fixer.fix(indexFile)
	require.NoError(t, err)

	// The broken link remains (healing is skipped for ambiguity).
	require.Len(t, res.BrokenLinks, 1)
	require.Empty(t, res.LinksUpdated)
	require.Len(t, res.HealSkipped, 1)
	require.Contains(t, res.HealSkipped[0].Reason, "ambiguous")
	require.Len(t, res.HealSkipped[0].Candidates, 2)

	// Link target should remain unchanged.
	// #nosec G304 -- test reads from a tempdir path
	data, err := os.ReadFile(indexFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "[Foo](FOO.md)")
}

func TestFixer_HealsBrokenLinks_ToFinalPath_WhenFixerAlsoRenamesDestination(t *testing.T) {
	repoDir := initGitRepo(t)
	docsDir := filepath.Join(repoDir, "docs")
	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "subdir"), 0o750))

	oldTarget := filepath.Join(docsDir, "file.md")
	indexFile := filepath.Join(docsDir, "index.md")
	require.NoError(t, os.WriteFile(oldTarget, []byte("# Target\n"), 0o600))
	require.NoError(t, os.WriteFile(indexFile, []byte("[Go](file.md)\n"), 0o600))

	git(t, repoDir, "add", "docs/file.md", "docs/index.md")
	git(t, repoDir, "commit", "-m", "add docs")

	// User moves the file into a subdir with an uppercase filename.
	git(t, repoDir, "mv", "docs/file.md", "docs/subdir/File.md")

	// Sanity: link is currently broken.
	before, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)
	require.Len(t, before, 1)

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, true)
	res, err := fixer.fix(docsDir)
	require.NoError(t, err)

	// Destination should be normalized by the fixer.
	finalTarget := filepath.Join(docsDir, "subdir", "file.md")
	require.FileExists(t, finalTarget)

	// Healing should update links to the FINAL path (subdir/file.md), not the
	// intermediate Git rename destination (subdir/File.md).
	// #nosec G304 -- test reads from a tempdir path
	data, err := os.ReadFile(indexFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "[Go](subdir/file.md)")
	require.NotContains(t, string(data), "subdir/File.md")

	// And the broken link worklist should be fully healed.
	require.Empty(t, res.BrokenLinks)
}
