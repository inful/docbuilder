package lint

import (
	"os"
	"path/filepath"
	"testing"

	helpers "git.home.luguber.info/inful/docbuilder/internal/testutil/testutils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixer_HealBrokenLinks(t *testing.T) {
	// Setup temporary directory and git repo
	repo, w, tempDir := helpers.SetupTestGitRepo(t)

	// 1. Create a file and commit it
	oldFile := filepath.Join(tempDir, "target.md")
	err := os.WriteFile(oldFile, []byte("# Target File\nContent"), 0o600)
	require.NoError(t, err)

	_, err = w.Add("target.md")
	require.NoError(t, err)

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)

	// 2. Rename the file manually and commit
	newFile := filepath.Join(tempDir, "moved_target.md")
	err = os.Rename(oldFile, newFile)
	require.NoError(t, err)

	_, err = w.Add("target.md") // Mark as deleted
	require.NoError(t, err)
	_, err = w.Add("moved_target.md") // Mark as added
	require.NoError(t, err)

	_, err = w.Commit("Move target file", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)

	// 3. Create a file with a broken link to the old path
	sourceFile := filepath.Join(tempDir, "index.md")
	err = os.WriteFile(sourceFile, []byte("# Index\n[Link](./target.md)"), 0o600)
	require.NoError(t, err)

	// 4. Initialize Fixer in the temp directory
	// We need to override the repo discovery for testing
	linter := NewLinter(nil)
	fixer := &Fixer{
		linter:   linter,
		gitRepo:  repo,
		gitAware: true,
		nowFn:    nil,
	}

	// 5. Run healing
	result := &FixResult{}
	fixer.healBrokenLinks(result, nil, tempDir)

	// 6. Verify link was updated
	// #nosec G304 -- sourceFile is deterministic in tests
	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "[Link](moved_target.md)")
	assert.Len(t, result.LinksUpdated, 1)
	assert.Equal(t, "./target.md", result.LinksUpdated[0].OldTarget)
	assert.Equal(t, "moved_target.md", result.LinksUpdated[0].NewTarget)
}

func TestFixer_UpdateLinkInFile(t *testing.T) {
	tempDir := t.TempDir()
	fixer := &Fixer{}

	t.Run("successful update", func(t *testing.T) {
		sourceFile := filepath.Join(tempDir, "source.md")
		originalContent := "# Title\n[Link](old.md)\nSome more text."
		err := os.WriteFile(sourceFile, []byte(originalContent), 0o600)
		require.NoError(t, err)

		bl := BrokenLink{
			SourceFile: sourceFile,
			Target:     "old.md",
			FullMatch:  "[Link](old.md)",
		}

		err = fixer.updateLinkInFile(bl, "new.md")
		require.NoError(t, err)

		// #nosec G304 -- test file
		content, err := os.ReadFile(sourceFile)
		require.NoError(t, err)
		assert.Equal(t, "# Title\n[Link](new.md)\nSome more text.", string(content))
	})

	t.Run("multiple occurrences of exactly same match", func(t *testing.T) {
		sourceFile := filepath.Join(tempDir, "multiple_exact.md")
		originalContent := "[Link](old.md) and another [Link](old.md)."
		err := os.WriteFile(sourceFile, []byte(originalContent), 0o600)
		require.NoError(t, err)

		bl := BrokenLink{
			SourceFile: sourceFile,
			Target:     "old.md",
			FullMatch:  "[Link](old.md)",
		}

		err = fixer.updateLinkInFile(bl, "new.md")
		require.NoError(t, err)

		// #nosec G304 -- test file
		content, err := os.ReadFile(sourceFile)
		require.NoError(t, err)
		assert.Equal(t, "[Link](new.md) and another [Link](new.md).", string(content))
	})

	t.Run("multiple occurrences with different text", func(t *testing.T) {
		sourceFile := filepath.Join(tempDir, "multiple_diff.md")
		originalContent := "[Link1](old.md) and [Link2](old.md)."
		err := os.WriteFile(sourceFile, []byte(originalContent), 0o600)
		require.NoError(t, err)

		// First match
		bl1 := BrokenLink{
			SourceFile: sourceFile,
			Target:     "old.md",
			FullMatch:  "[Link1](old.md)",
		}
		err = fixer.updateLinkInFile(bl1, "new.md")
		require.NoError(t, err)

		// Second match
		bl2 := BrokenLink{
			SourceFile: sourceFile,
			Target:     "old.md",
			FullMatch:  "[Link2](old.md)",
		}
		err = fixer.updateLinkInFile(bl2, "new.md")
		require.NoError(t, err)

		// #nosec G304 -- test file
		content, err := os.ReadFile(sourceFile)
		require.NoError(t, err)
		assert.Equal(t, "[Link1](new.md) and [Link2](new.md).", string(content))
	})

	t.Run("preserve fragment", func(t *testing.T) {
		sourceFile := filepath.Join(tempDir, "fragment.md")
		originalContent := "[Link](old.md#section)"
		err := os.WriteFile(sourceFile, []byte(originalContent), 0o600)
		require.NoError(t, err)

		bl := BrokenLink{
			SourceFile: sourceFile,
			Target:     "old.md#section",
			FullMatch:  "[Link](old.md#section)",
		}

		err = fixer.updateLinkInFile(bl, "new.md")
		require.NoError(t, err)

		// #nosec G304 -- test file
		content, err := os.ReadFile(sourceFile)
		require.NoError(t, err)
		assert.Equal(t, "[Link](new.md#section)", string(content))
	})

	t.Run("link not found error", func(t *testing.T) {
		sourceFile := filepath.Join(tempDir, "notfound.md")
		originalContent := "[Link](other.md)"
		err := os.WriteFile(sourceFile, []byte(originalContent), 0o600)
		require.NoError(t, err)

		bl := BrokenLink{
			SourceFile: sourceFile,
			Target:     "old.md",
			FullMatch:  "[Link](old.md)",
		}

		err = fixer.updateLinkInFile(bl, "new.md")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "link not found in file")
	})

	t.Run("reference-style link", func(t *testing.T) {
		sourceFile := filepath.Join(tempDir, "reference.md")
		originalContent := "# Ref\n[id]: old.md\n"
		err := os.WriteFile(sourceFile, []byte(originalContent), 0o600)
		require.NoError(t, err)

		bl := BrokenLink{
			SourceFile: sourceFile,
			Target:     "old.md",
			FullMatch:  "[id]: old.md",
			LinkType:   LinkTypeReference,
		}

		err = fixer.updateLinkInFile(bl, "new.md")
		require.NoError(t, err)

		// #nosec G304 -- test file
		content, err := os.ReadFile(sourceFile)
		require.NoError(t, err)
		assert.Equal(t, "# Ref\n[id]: new.md\n", string(content))
	})

	t.Run("mixed styles in one file", func(t *testing.T) {
		sourceFile := filepath.Join(tempDir, "mixed.md")
		originalContent := "[Inline](old.md)\n\n[id]: old.md"
		err := os.WriteFile(sourceFile, []byte(originalContent), 0o600)
		require.NoError(t, err)

		// Fix inline
		err = fixer.updateLinkInFile(BrokenLink{
			SourceFile: sourceFile,
			Target:     "old.md",
			FullMatch:  "[Inline](old.md)",
		}, "new.md")
		require.NoError(t, err)

		// Fix reference
		err = fixer.updateLinkInFile(BrokenLink{
			SourceFile: sourceFile,
			Target:     "old.md",
			FullMatch:  "[id]: old.md",
		}, "new.md")
		require.NoError(t, err)

		// #nosec G304 -- test file
		content, err := os.ReadFile(sourceFile)
		require.NoError(t, err)
		assert.Equal(t, "[Inline](new.md)\n\n[id]: new.md", string(content))
	})
}

func TestFixer_FindSameContentFile(t *testing.T) {
	repo, w, tempDir := helpers.SetupTestGitRepo(t)

	signature := &object.Signature{Name: "Test", Email: "test@example.com"}

	// 1. Initial commit with two files
	f1Path := "file1.md"
	f2Path := "file2.md"
	content1 := []byte("Initial content")
	content2 := []byte("Different content")

	err := os.WriteFile(filepath.Join(tempDir, f1Path), content1, 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, f2Path), content2, 0o600)
	require.NoError(t, err)

	_, err = w.Add(f1Path)
	require.NoError(t, err)
	_, err = w.Add(f2Path)
	require.NoError(t, err)

	initialCommitHash, err := w.Commit("Initial commit", &git.CommitOptions{Author: signature})
	require.NoError(t, err)

	fixer := &Fixer{}

	t.Run("finds same content file after rename", func(t *testing.T) {
		// Second commit: Rename file1.md to renamed.md
		newName := "renamed.md"
		err = os.Rename(filepath.Join(tempDir, f1Path), filepath.Join(tempDir, newName))
		require.NoError(t, err)

		_, err = w.Add(f1Path) // Delete old
		require.NoError(t, err)
		_, err = w.Add(newName) // Add new
		require.NoError(t, err)

		secondCommitHash, err := w.Commit("Rename file", &git.CommitOptions{Author: signature})
		require.NoError(t, err)

		secondCommit, err := repo.CommitObject(secondCommitHash)
		require.NoError(t, err)

		found := fixer.findSameContentFile(secondCommit, f1Path, tempDir)
		assert.Equal(t, filepath.Join(tempDir, newName), found)
	})

	t.Run("returns empty if content is different", func(t *testing.T) {
		// Rename file2.md BUT change content too
		err = os.Rename(filepath.Join(tempDir, f2Path), filepath.Join(tempDir, "renamed2.md"))
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "renamed2.md"), []byte("Now it's different"), 0o600)
		require.NoError(t, err)

		_, err = w.Add(f2Path)
		require.NoError(t, err)
		_, err = w.Add("renamed2.md")
		require.NoError(t, err)

		thirdCommitHash, err := w.Commit("Change content during rename", &git.CommitOptions{Author: signature})
		require.NoError(t, err)

		thirdCommit, err := repo.CommitObject(thirdCommitHash)
		require.NoError(t, err)

		// Search for content of file2.md as it was in initialCommit (via its parent)
		found := fixer.findSameContentFile(thirdCommit, f2Path, tempDir)
		assert.Empty(t, found)
	})

	t.Run("returns empty if no parent commit", func(t *testing.T) {
		initialCommit, err := repo.CommitObject(initialCommitHash)
		require.NoError(t, err)

		found := fixer.findSameContentFile(initialCommit, f1Path, tempDir)
		assert.Empty(t, found)
	})

	t.Run("returns empty if file not in parent", func(t *testing.T) {
		// Create a new commit where we add a file that wasn't there before
		err = os.WriteFile(filepath.Join(tempDir, "newfile.md"), []byte("New file content"), 0o600)
		require.NoError(t, err)
		_, err = w.Add("newfile.md")
		require.NoError(t, err)

		newCommitHash, err := w.Commit("Add new file", &git.CommitOptions{Author: signature})
		require.NoError(t, err)

		newCommit, err := repo.CommitObject(newCommitHash)
		require.NoError(t, err)

		// Search for a path that didn't exist in parent
		found := fixer.findSameContentFile(newCommit, "something_else.md", tempDir)
		assert.Empty(t, found)
	})

	t.Run("finds one among multiple copies", func(t *testing.T) {
		// Commit two files with exact same content
		sameContent := []byte("Identical content")
		err = os.WriteFile(filepath.Join(tempDir, "copy1.md"), sameContent, 0o600)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "copy2.md"), sameContent, 0o600)
		require.NoError(t, err)

		_, err = w.Add("copy1.md")
		require.NoError(t, err)
		_, err = w.Add("copy2.md")
		require.NoError(t, err)

		_, err = w.Commit("Base for copies", &git.CommitOptions{Author: signature})
		require.NoError(t, err)

		// Remove copy1.md in next commit
		err = os.Remove(filepath.Join(tempDir, "copy1.md"))
		require.NoError(t, err)
		_, err = w.Add("copy1.md")
		require.NoError(t, err)

		nextHash, err := w.Commit("Remove copy1", &git.CommitOptions{Author: signature})
		require.NoError(t, err)

		nextCommit, err := repo.CommitObject(nextHash)
		require.NoError(t, err)

		found := fixer.findSameContentFile(nextCommit, "copy1.md", tempDir)
		assert.Equal(t, filepath.Join(tempDir, "copy2.md"), found)
	})
}
