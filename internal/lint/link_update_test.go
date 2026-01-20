package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPILink = "[API](./api-guide.md)"

// TestApplyLinkUpdates_BasicUpdate tests updating a single link in a single file.
func TestApplyLinkUpdates_BasicUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.md")
	targetFile := filepath.Join(tmpDir, "target.md")

	// Create source file with a link
	sourceContent := `# Documentation

See [API Guide](./api-guide.md) for details.
`
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0o600))
	require.NoError(t, os.WriteFile(targetFile, []byte("# API Guide"), 0o600))

	// Create link reference
	links := []LinkReference{
		{
			SourceFile: sourceFile,
			LineNumber: 3,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeInline,
		},
	}

	// Apply updates
	fixer := &Fixer{}
	oldPath := filepath.Join(tmpDir, "api-guide.md")
	newPath := filepath.Join(tmpDir, "api_guide.md")

	updates, err := fixer.applyLinkUpdates(links, oldPath, newPath)
	require.NoError(t, err)

	// Verify update was recorded
	assert.Len(t, updates, 1)
	assert.Equal(t, sourceFile, updates[0].SourceFile)
	assert.Equal(t, 3, updates[0].LineNumber)
	assert.Equal(t, "./api-guide.md", updates[0].OldTarget)
	assert.Equal(t, "./api_guide.md", updates[0].NewTarget)

	// Verify file was updated
	// #nosec G304 -- test utility reading from test output directory
	// #nosec G304 -- test utility reading from test output directory
	updatedContent, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	assert.Contains(t, string(updatedContent), "[API Guide](./api_guide.md)")
	assert.NotContains(t, string(updatedContent), "[API Guide](./api-guide.md)")

	// Verify backup was cleaned up
	backupPath := sourceFile + ".backup"
	_, err = os.Stat(backupPath)
	assert.True(t, os.IsNotExist(err), "backup file should be removed on success")
}

// TestApplyLinkUpdates_MultipleLinksInFile tests updating multiple links in a single file.
func TestApplyLinkUpdates_MultipleLinksInFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.md")

	// Create source file with multiple links
	sourceContent := `# Documentation

See [API Guide](./api-guide.md) for details.

Also check [API Reference](../api-guide.md) and ![Screenshot](./images/api-guide.md.png).

Reference: [api-guide][1]

[1]: ./api-guide.md
`
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0o600))

	// Create link references (will be sorted reverse by applyLinkUpdates)
	links := []LinkReference{
		{
			SourceFile: sourceFile,
			LineNumber: 3,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: sourceFile,
			LineNumber: 5,
			Target:     "../api-guide.md",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: sourceFile,
			LineNumber: 5,
			Target:     "./images/api-guide.md.png",
			LinkType:   LinkTypeImage,
		},
		{
			SourceFile: sourceFile,
			LineNumber: 9,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeReference,
		},
	}

	// Apply updates
	fixer := &Fixer{}
	oldPath := filepath.Join(tmpDir, "api-guide.md")
	newPath := filepath.Join(tmpDir, "api_guide.md")

	updates, err := fixer.applyLinkUpdates(links, oldPath, newPath)
	require.NoError(t, err)

	// Verify all updates were recorded
	assert.Len(t, updates, 4)

	// Verify file was updated correctly
	// #nosec G304 -- test utility reading from test output directory
	updatedContent, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	content := string(updatedContent)

	assert.Contains(t, content, "[API Guide](./api_guide.md)")
	assert.Contains(t, content, "[API Reference](../api_guide.md)")
	assert.Contains(t, content, "![Screenshot](./images/api_guide.md.png)")
	assert.Contains(t, content, "[1]: ./api_guide.md")

	// Verify old links are gone
	assert.NotContains(t, content, "./api-guide.md")
	assert.NotContains(t, content, "../api-guide.md")
}

// TestApplyLinkUpdates_MultipleSourceFiles tests updating links across multiple files.
func TestApplyLinkUpdates_MultipleSourceFiles(t *testing.T) {
	tmpDir := t.TempDir()
	source1 := filepath.Join(tmpDir, "guide1.md")
	source2 := filepath.Join(tmpDir, "guide2.md")

	// Create source files
	require.NoError(t, os.WriteFile(source1, []byte(testAPILink), 0o600))
	require.NoError(t, os.WriteFile(source2, []byte(testAPILink), 0o600))

	// Create link references from multiple files
	links := []LinkReference{
		{
			SourceFile: source1,
			LineNumber: 1,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: source2,
			LineNumber: 1,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeInline,
		},
	}

	// Apply updates
	fixer := &Fixer{}
	oldPath := filepath.Join(tmpDir, "api-guide.md")
	newPath := filepath.Join(tmpDir, "api_guide.md")

	updates, err := fixer.applyLinkUpdates(links, oldPath, newPath)
	require.NoError(t, err)

	// Verify both updates were recorded
	assert.Len(t, updates, 2)

	// Verify both files were updated
	// #nosec G304 -- test utility reading from test output directory
	content1, err := os.ReadFile(source1)
	require.NoError(t, err)
	assert.Contains(t, string(content1), "./api_guide.md")

	// #nosec G304 -- test utility reading from test output directory
	content2, err := os.ReadFile(source2)
	require.NoError(t, err)
	assert.Contains(t, string(content2), "./api_guide.md")
}

// TestApplyLinkUpdates_RelativePathPreservation tests that relative paths are preserved.
func TestApplyLinkUpdates_RelativePathPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "docs", "source.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(sourceFile), 0o750))

	// Create source file with various relative paths
	sourceContent := `# Links

- [Same dir](./api-guide.md)
- [Parent](../api-guide.md)
- [Subdir](./sub/api-guide.md)
- [No prefix](api-guide.md)
`
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0o600))

	// Create link references with different path styles
	links := []LinkReference{
		{
			SourceFile: sourceFile,
			LineNumber: 3,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: sourceFile,
			LineNumber: 4,
			Target:     "../api-guide.md",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: sourceFile,
			LineNumber: 5,
			Target:     "./sub/api-guide.md",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: sourceFile,
			LineNumber: 6,
			Target:     "api-guide.md",
			LinkType:   LinkTypeInline,
		},
	}

	// Apply updates
	fixer := &Fixer{}
	oldPath := filepath.Join(tmpDir, "api-guide.md")
	newPath := filepath.Join(tmpDir, "api_guide.md")

	updates, err := fixer.applyLinkUpdates(links, oldPath, newPath)
	require.NoError(t, err)

	// Verify all updates preserve relative path structure
	assert.Len(t, updates, 4)

	// #nosec G304 -- test utility reading from test output directory
	updatedContent, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	content := string(updatedContent)

	assert.Contains(t, content, "(./api_guide.md)")
	assert.Contains(t, content, "(../api_guide.md)")
	assert.Contains(t, content, "(./sub/api_guide.md)")
	assert.Contains(t, content, "(api_guide.md)")
}

// TestApplyLinkUpdates_AnchorFragmentPreservation tests that anchor fragments are preserved.
func TestApplyLinkUpdates_AnchorFragmentPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.md")

	// Create source file with links containing anchors
	sourceContent := `# Documentation

See [Overview](./api-guide.md#overview) and [Methods](./api-guide.md#methods).
`
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0o600))

	// Create link references with anchors
	links := []LinkReference{
		{
			SourceFile: sourceFile,
			LineNumber: 3,
			Target:     "./api-guide.md#overview",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: sourceFile,
			LineNumber: 3,
			Target:     "./api-guide.md#methods",
			LinkType:   LinkTypeInline,
		},
	}

	// Apply updates
	fixer := &Fixer{}
	oldPath := filepath.Join(tmpDir, "api-guide.md")
	newPath := filepath.Join(tmpDir, "api_guide.md")

	updates, err := fixer.applyLinkUpdates(links, oldPath, newPath)
	require.NoError(t, err)

	// Verify anchors are preserved
	assert.Len(t, updates, 2)
	assert.Equal(t, "./api_guide.md#overview", updates[0].NewTarget)
	assert.Equal(t, "./api_guide.md#methods", updates[1].NewTarget)

	// #nosec G304 -- test utility reading from test output directory
	updatedContent, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	content := string(updatedContent)

	assert.Contains(t, content, "[Overview](./api_guide.md#overview)")
	assert.Contains(t, content, "[Methods](./api_guide.md#methods)")
}

// TestApplyLinkUpdates_AtomicRollback tests that updates are rolled back on error.
func TestApplyLinkUpdates_AtomicRollback(t *testing.T) {
	tmpDir := t.TempDir()
	source1 := filepath.Join(tmpDir, "source1.md")
	source2 := filepath.Join(tmpDir, "source2.md")

	// Create first source file (valid)
	originalContent1 := testAPILink
	require.NoError(t, os.WriteFile(source1, []byte(originalContent1), 0o600))

	// Create second source file as read-only to trigger error
	originalContent2 := testAPILink
	require.NoError(t, os.WriteFile(source2, []byte(originalContent2), 0o000)) // No write permission

	// Create link references
	links := []LinkReference{
		{
			SourceFile: source1,
			LineNumber: 1,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: source2,
			LineNumber: 1,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeInline,
		},
	}

	// Apply updates (should fail on source2)
	fixer := &Fixer{}
	oldPath := filepath.Join(tmpDir, "api-guide.md")
	newPath := filepath.Join(tmpDir, "api_guide.md")

	updates, err := fixer.applyLinkUpdates(links, oldPath, newPath)

	// Verify error was returned
	assert.Error(t, err)
	assert.Nil(t, updates)

	// Verify first file was rolled back to original content
	// #nosec G304 -- test utility reading from test output directory
	content1, err := os.ReadFile(source1)
	require.NoError(t, err)
	assert.Equal(t, originalContent1, string(content1), "file should be rolled back on error")

	// Verify no backup files remain
	backup1 := source1 + ".backup"
	_, err = os.Stat(backup1)
	assert.True(t, os.IsNotExist(err), "backup file should be cleaned up")

	// Clean up read-only file
	// #nosec G302 -- intentional permission change for test cleanup
	_ = os.Chmod(source2, 0o600)
}

// TestApplyLinkUpdates_EmptyLinks tests behavior with no links to update.
func TestApplyLinkUpdates_EmptyLinks(t *testing.T) {
	fixer := &Fixer{}
	updates, err := fixer.applyLinkUpdates([]LinkReference{}, "/tmp/old.md", "/tmp/new.md")
	require.NoError(t, err)
	assert.Empty(t, updates)
}

// TestUpdateLinkTarget tests the link target transformation logic.
func TestUpdateLinkTarget(t *testing.T) {
	fixer := &Fixer{}

	tests := []struct {
		name       string
		oldPath    string
		newPath    string
		linkTarget string
		expected   string
	}{
		{
			name:       "same directory",
			oldPath:    "/docs/api-guide.md",
			newPath:    "/docs/api_guide.md",
			linkTarget: "./api-guide.md",
			expected:   "./api_guide.md",
		},
		{
			name:       "parent directory",
			oldPath:    "/docs/api-guide.md",
			newPath:    "/docs/api_guide.md",
			linkTarget: "../api-guide.md",
			expected:   "../api_guide.md",
		},
		{
			name:       "subdirectory",
			oldPath:    "/docs/api-guide.md",
			newPath:    "/docs/api_guide.md",
			linkTarget: "./sub/api-guide.md",
			expected:   "./sub/api_guide.md",
		},
		{
			name:       "no directory prefix",
			oldPath:    "/docs/api-guide.md",
			newPath:    "/docs/api_guide.md",
			linkTarget: "api-guide.md",
			expected:   "api_guide.md",
		},
		{
			name:       "with anchor",
			oldPath:    "/docs/api-guide.md",
			newPath:    "/docs/api_guide.md",
			linkTarget: "./api-guide.md#section",
			expected:   "./api_guide.md#section",
		},
		{
			name:       "complex path",
			oldPath:    "/docs/guides/api-guide.md",
			newPath:    "/docs/guides/api_guide.md",
			linkTarget: "../../docs/guides/api-guide.md",
			expected:   "../../docs/guides/api_guide.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linkRef := LinkReference{
				Target: tt.linkTarget,
			}
			result := fixer.updateLinkTarget(linkRef, tt.oldPath, tt.newPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIntegration_RenameWithLinkUpdates tests the full integration of renaming and link updates.
func TestIntegration_RenameWithLinkUpdates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a documentation structure with a file that violates naming conventions
	apiFile := filepath.Join(tmpDir, "API_Guide.md") // Uppercase - violates kebab-case
	indexFile := filepath.Join(tmpDir, "index.md")
	readmeFile := filepath.Join(tmpDir, "docs", "README.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(readmeFile), 0o750))

	// Create files with cross-references
	require.NoError(t, os.WriteFile(apiFile, []byte("# API Guide"), 0o600))
	require.NoError(t, os.WriteFile(indexFile, []byte(`# Index
See [API Guide](./API_Guide.md) for details.
`), 0o600))
	require.NoError(t, os.WriteFile(readmeFile, []byte(`# Docs
Check [API](../API_Guide.md).
`), 0o600))

	// Create linter and fixer
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false) // not dry-run, no confirm

	// Apply fixes
	result, err := fixer.Fix(tmpDir)
	require.NoError(t, err)
	require.Empty(t, result.Errors, "fix should succeed")

	// Verify file was renamed
	assert.Equal(t, 1, len(result.FilesRenamed))
	newPath := filepath.Join(tmpDir, "api_guide.md") // Expected kebab-case name
	_, statErr := os.Stat(newPath)
	require.NoError(t, statErr, "renamed file should exist")

	// Verify links were updated
	assert.GreaterOrEqual(t, len(result.LinksUpdated), 2, "should update links in index and readme")

	// Verify index.md was updated
	// #nosec G304 -- test utility reading from test output directory
	indexContent, err := os.ReadFile(indexFile)
	require.NoError(t, err)
	assert.Contains(t, string(indexContent), "[API Guide](./api_guide.md)")
	assert.NotContains(t, string(indexContent), "./API_Guide.md")

	// Verify docs/README.md was updated
	// #nosec G304 -- test utility reading from test output directory
	readmeContent, err := os.ReadFile(readmeFile)
	require.NoError(t, err)
	assert.Contains(t, string(readmeContent), "[API](../api_guide.md)")
	assert.NotContains(t, string(readmeContent), "../API_Guide.md")

	// Verify summary includes link updates
	summary := result.Summary()
	assert.Contains(t, summary, "Links updated:")
	assert.Contains(t, summary, "index.md")
	assert.Contains(t, summary, "README.md")
}

// TestApplyLinkUpdates_PreservesAnchorFragments tests that anchor fragments (#section) are preserved.
func TestApplyLinkUpdates_PreservesAnchorFragments(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.md")

	// Create source file with links that have anchor fragments
	sourceContent := `# Documentation

See [Authentication](./api-guide.md#authentication) for auth details.

Also check [Overview](./api-guide.md#overview) section.

Reference to [Errors](../api-guide.md#errors).
`
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0o600))

	// Create link references with anchor fragments
	links := []LinkReference{
		{
			SourceFile: sourceFile,
			LineNumber: 3,
			Target:     "./api-guide.md",
			Fragment:   "#authentication",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: sourceFile,
			LineNumber: 5,
			Target:     "./api-guide.md",
			Fragment:   "#overview",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: sourceFile,
			LineNumber: 7,
			Target:     "../api-guide.md",
			Fragment:   "#errors",
			LinkType:   LinkTypeInline,
		},
	}

	// Apply updates
	fixer := &Fixer{}
	oldPath := filepath.Join(tmpDir, "api-guide.md")
	newPath := filepath.Join(tmpDir, "api_guide.md")

	updates, err := fixer.applyLinkUpdates(links, oldPath, newPath)
	require.NoError(t, err)

	// Verify all updates were recorded with fragments preserved
	// Updates may come in any order due to internal sorting, so check by content
	assert.Len(t, updates, 3)

	// Create a map of line numbers to updates for order-independent verification
	updatesByLine := make(map[int]LinkUpdate)
	for _, update := range updates {
		updatesByLine[update.LineNumber] = update
	}

	// Verify each expected update exists at the correct line
	assert.Contains(t, updatesByLine, 3)
	assert.Equal(t, "./api-guide.md#authentication", updatesByLine[3].OldTarget)
	assert.Equal(t, "./api_guide.md#authentication", updatesByLine[3].NewTarget)

	assert.Contains(t, updatesByLine, 5)
	assert.Equal(t, "./api-guide.md#overview", updatesByLine[5].OldTarget)
	assert.Equal(t, "./api_guide.md#overview", updatesByLine[5].NewTarget)

	assert.Contains(t, updatesByLine, 7)
	assert.Equal(t, "../api-guide.md#errors", updatesByLine[7].OldTarget)
	assert.Equal(t, "../api_guide.md#errors", updatesByLine[7].NewTarget)

	// Verify file was updated correctly with fragments preserved
	// #nosec G304 -- test utility reading from test output directory
	updatedContent, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	content := string(updatedContent)

	assert.Contains(t, content, "[Authentication](./api_guide.md#authentication)")
	assert.Contains(t, content, "[Overview](./api_guide.md#overview)")
	assert.Contains(t, content, "[Errors](../api_guide.md#errors)")

	// Verify old links are gone
	assert.NotContains(t, content, "./api-guide.md")
	assert.NotContains(t, content, "../api-guide.md")
}

// TestApplyLinkUpdates_RollbackOnFailure tests that changes are rolled back on failure.
func TestApplyLinkUpdates_RollbackOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	source1 := filepath.Join(tmpDir, "source1.md")
	source2 := filepath.Join(tmpDir, "source2.md")

	// Create first source file
	originalContent1 := testAPILink
	require.NoError(t, os.WriteFile(source1, []byte(originalContent1), 0o600))

	// Create second source file as read-only to trigger failure
	originalContent2 := testAPILink
	require.NoError(t, os.WriteFile(source2, []byte(originalContent2), 0o600))
	// #nosec G302 -- intentional read-only permission for test setup
	require.NoError(t, os.Chmod(source2, 0o444)) // Make it read-only
	defer func() {
		// #nosec G302 -- intentional permission change for test cleanup
		_ = os.Chmod(source2, 0o600) // Clean up (ignore error)
	}()

	// Create link references - source1 will succeed, source2 will fail
	links := []LinkReference{
		{
			SourceFile: source1,
			LineNumber: 1,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeInline,
		},
		{
			SourceFile: source2,
			LineNumber: 1,
			Target:     "./api-guide.md",
			LinkType:   LinkTypeInline,
		},
	}

	// Apply updates - should fail and rollback
	fixer := &Fixer{}
	oldPath := filepath.Join(tmpDir, "api-guide.md")
	newPath := filepath.Join(tmpDir, "api_guide.md")

	_, err := fixer.applyLinkUpdates(links, oldPath, newPath)
	require.Error(t, err, "should fail when writing to read-only file")

	// Verify source1 was rolled back to original content
	// #nosec G304 -- test utility reading from test output directory
	content1, err := os.ReadFile(source1)
	require.NoError(t, err)
	assert.Equal(t, originalContent1, string(content1), "source1 should be rolled back")

	// Verify backup was cleaned up
	backup1 := source1 + ".backup"
	_, err = os.Stat(backup1)
	assert.True(t, os.IsNotExist(err), "backup should be removed after rollback")
}
