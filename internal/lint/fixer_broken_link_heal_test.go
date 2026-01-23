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

func TestFixer_HealsBrokenLinks_RenameBackRoundTrip(t *testing.T) {
	for _, newName := range []string{"HubbaBubba.md", "something.md"} {
		t.Run(newName, func(t *testing.T) {
			repoDir := initGitRepo(t)
			docsDir := filepath.Join(repoDir, "docs")
			require.NoError(t, os.MkdirAll(docsDir, 0o750))

			target := filepath.Join(docsDir, "test.md")
			linkFile := filepath.Join(docsDir, "link.md")

			require.NoError(t, os.WriteFile(target, []byte("# Test\n"), 0o600))
			require.NoError(t, os.WriteFile(linkFile, []byte("[Test](test.md)\n"), 0o600))

			git(t, repoDir, "add", "docs/test.md", "docs/link.md")
			git(t, repoDir, "commit", "-m", "add docs")

			// User moves the file (staged git rename) and forgets to update links.
			// Staging ensures rename detection remains reliable even if the fixer
			// modifies the destination file contents during other fix phases.
			git(t, repoDir, "mv", "docs/test.md", "docs/"+newName)

			linter := NewLinter(&Config{Format: "text"})
			fixer := NewFixer(linter, false, true)

			// First run: healer should update link to point at the renamed file.
			res, err := fixer.fix(docsDir)
			require.NoError(t, err)
			require.Empty(t, res.BrokenLinks)

			// The fixer may further normalize the renamed filename (e.g. casing).
			// Resolve the current name on disk by finding the non-link markdown file.
			entries, err := os.ReadDir(docsDir)
			require.NoError(t, err)
			finalName := ""
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if name == "link.md" {
					continue
				}
				if filepath.Ext(name) != ".md" {
					continue
				}
				finalName = name
				break
			}
			require.NotEmpty(t, finalName, "expected renamed target markdown file to exist")
			require.FileExists(t, filepath.Join(docsDir, finalName))

			// #nosec G304 -- test reads from a tempdir path
			data, err := os.ReadFile(linkFile)
			require.NoError(t, err)
			require.Contains(t, string(data), "[Test]("+finalName+")")

			// Clear staged changes so the subsequent rename-back produces no
			// uncommitted rename mappings (exercises the HEAD fallback behavior).
			git(t, repoDir, "reset")

			// User renames the file back (filesystem rename) and again forgets to update links.
			require.NoError(t, os.Rename(filepath.Join(docsDir, finalName), filepath.Join(docsDir, "test.md")))

			// Second run: healer should update link back to test.md.
			res2, err := fixer.fix(docsDir)
			require.NoError(t, err)
			require.Empty(t, res2.BrokenLinks)

			// #nosec G304 -- test reads from a tempdir path
			data2, err := os.ReadFile(linkFile)
			require.NoError(t, err)
			require.Contains(t, string(data2), "[Test](test.md)")
		})
	}
}

func TestFixer_HealsBrokenLinks_PreservesLabelWhenLabelEqualsOldDestination(t *testing.T) {
	repoDir := initGitRepo(t)
	docsDir := filepath.Join(repoDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	target := filepath.Join(docsDir, "file.md")
	linkFile := filepath.Join(docsDir, "link.md")

	require.NoError(t, os.WriteFile(target, []byte("# File\n"), 0o600))
	require.NoError(t, os.WriteFile(linkFile, []byte("[file.md](file.md)\n"), 0o600))

	git(t, repoDir, "add", "docs/file.md", "docs/link.md")
	git(t, repoDir, "commit", "-m", "add docs")

	// User renames the file and forgets to update the link.
	git(t, repoDir, "mv", "docs/file.md", "docs/file-rename.md")

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, true)

	res, err := fixer.fix(docsDir)
	require.NoError(t, err)
	require.Empty(t, res.BrokenLinks)

	// #nosec G304 -- test reads from a tempdir path
	data, err := os.ReadFile(linkFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "[file.md](file-rename.md)")
	require.NotContains(t, string(data), "[file-rename.md](file-rename.md)")

	// User renames the file back and forgets to update the link.
	git(t, repoDir, "mv", "docs/file-rename.md", "docs/file.md")

	res2, err := fixer.fix(docsDir)
	require.NoError(t, err)
	require.Empty(t, res2.BrokenLinks)

	// #nosec G304 -- test reads from a tempdir path
	data2, err := os.ReadFile(linkFile)
	require.NoError(t, err)
	require.Contains(t, string(data2), "[file.md](file.md)")
	require.NotContains(t, string(data2), "[file.md](file-rename.md)")
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

func TestFixer_WarnsOnRenameCollision_WhenHistoryRenameCreatesCaseOnlyConflict(t *testing.T) {
	repoDir := initGitRepo(t)
	docsDir := filepath.Join(repoDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	// Existing canonical file.
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "test.md"), []byte("# Test\n"), 0o600))

	// File that will be renamed (committed) to a conflicting case variant.
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "test2.md"), []byte("# Test2\n"), 0o600))

	// Link still points at the old name after the rename.
	indexFile := filepath.Join(docsDir, "index.md")
	require.NoError(t, os.WriteFile(indexFile, []byte("[Go](test2.md)\n"), 0o600))

	git(t, repoDir, "add", "docs/test.md", "docs/test2.md", "docs/index.md")
	git(t, repoDir, "commit", "-m", "add docs")

	// User performs a committed rename that introduces a case-only collision potential:
	// docs/test2.md -> docs/Test.md, while docs/test.md already exists.
	git(t, repoDir, "mv", "docs/test2.md", "docs/Test.md")
	git(t, repoDir, "commit", "-m", "rename test2 to Test")

	// Sanity: link is currently broken.
	before, err := detectBrokenLinks(docsDir)
	require.NoError(t, err)
	require.Len(t, before, 1)

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false) // force=false: must refuse overwrite
	res, err := fixer.fix(docsDir)
	require.NoError(t, err)

	// The broken link should be healed using history-derived rename mappings.
	require.Empty(t, res.BrokenLinks)
	require.NotEmpty(t, res.LinksUpdated)

	// But filename normalization (Test.md -> test.md) must fail due to collision,
	// and the user must be warned (error recorded).
	require.NotEmpty(t, res.FilesRenamed)
	require.False(t, res.FilesRenamed[0].Success)
	require.NotNil(t, res.FilesRenamed[0].Error)
	require.NotEmpty(t, res.Errors)

	// The link should point to the existing on-disk destination (Test.md), since
	// normalization could not be applied.
	// #nosec G304 -- test reads from a tempdir path
	data, err := os.ReadFile(indexFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "[Go](Test.md)")
}
