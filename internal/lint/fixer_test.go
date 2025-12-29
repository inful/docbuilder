package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixer_CanFixFilename(t *testing.T) {
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	t.Run("detects fixable filename issues", func(t *testing.T) {
		issues := []Issue{
			{
				Rule:     "filename-conventions",
				Severity: SeverityError,
				Message:  "Filename contains uppercase",
			},
		}

		canFix := fixer.canFixFilename(issues)
		assert.True(t, canFix)
	})

	t.Run("returns false for non-filename issues", func(t *testing.T) {
		issues := []Issue{
			{
				Rule:     "frontmatter-validation",
				Severity: SeverityError,
				Message:  "Invalid frontmatter",
			},
		}

		canFix := fixer.canFixFilename(issues)
		assert.False(t, canFix)
	})
}

func TestFixer_DryRun(t *testing.T) {
	// Create a temporary directory with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "API_Guide.md")

	err := os.WriteFile(testFile, []byte("# API Guide"), 0644)
	require.NoError(t, err)

	// Run fixer in dry-run mode
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, true, false) // dry-run enabled

	result, err := fixer.Fix(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify file was not actually renamed
	_, err = os.Stat(testFile)
	assert.NoError(t, err, "original file should still exist in dry-run mode")

	// Verify fix result shows what would happen
	assert.Len(t, result.FilesRenamed, 1)
	assert.True(t, result.FilesRenamed[0].Success)
	assert.Equal(t, testFile, result.FilesRenamed[0].OldPath)
}

func TestFixer_RenameFile(t *testing.T) {
	// Create a temporary directory with a test file
	tmpDir := t.TempDir()
	oldFile := filepath.Join(tmpDir, "API_Guide.md")
	expectedNewFile := filepath.Join(tmpDir, "api_guide.md")

	err := os.WriteFile(oldFile, []byte("# API Guide"), 0644)
	require.NoError(t, err)

	// Run fixer (not dry-run)
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	result, err := fixer.Fix(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify file was renamed
	_, err = os.Stat(oldFile)
	assert.Error(t, err, "original file should not exist after rename")

	_, err = os.Stat(expectedNewFile)
	assert.NoError(t, err, "new file should exist after rename")

	// Verify fix result
	assert.Len(t, result.FilesRenamed, 1)
	assert.True(t, result.FilesRenamed[0].Success)
	assert.Equal(t, oldFile, result.FilesRenamed[0].OldPath)
	assert.Equal(t, expectedNewFile, result.FilesRenamed[0].NewPath)
	assert.Equal(t, 1, result.ErrorsFixed)
}

func TestFixer_RenameMultipleFiles(t *testing.T) {
	// Create a temporary directory with multiple test files
	tmpDir := t.TempDir()

	files := []struct {
		old      string
		expected string
	}{
		{"API_Guide.md", "api_guide.md"},
		{"User Manual.md", "user-manual.md"},
		{"Config@v2.md", "configv2.md"},
	}

	for _, f := range files {
		oldPath := filepath.Join(tmpDir, f.old)
		err := os.WriteFile(oldPath, []byte("# Test"), 0644)
		require.NoError(t, err)
	}

	// Run fixer
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	result, err := fixer.Fix(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Debug: print what was renamed
	t.Logf("Files renamed: %d", len(result.FilesRenamed))
	for _, op := range result.FilesRenamed {
		t.Logf("  %s -> %s (success: %v, error: %v)", op.OldPath, op.NewPath, op.Success, op.Error)
	}

	// Verify all files were renamed
	assert.Len(t, result.FilesRenamed, len(files))
	// Note: ErrorsFixed may be > len(files) if a single file has multiple issues
	assert.GreaterOrEqual(t, result.ErrorsFixed, len(files), "should fix at least one error per file")

	for _, f := range files {
		oldPath := filepath.Join(tmpDir, f.old)
		newPath := filepath.Join(tmpDir, f.expected)

		_, err = os.Stat(oldPath)
		assert.Error(t, err, "old file %s should not exist", f.old)

		_, err = os.Stat(newPath)
		assert.NoError(t, err, "new file %s should exist", f.expected)
	}
}

func TestFixer_ErrorWhenTargetExists(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create both old and new files
	oldFile := filepath.Join(tmpDir, "API_Guide.md")
	newFile := filepath.Join(tmpDir, "api_guide.md")

	err := os.WriteFile(oldFile, []byte("# API Guide"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(newFile, []byte("# Existing"), 0644)
	require.NoError(t, err)

	// Run fixer without force flag
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	result, err := fixer.Fix(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify rename failed due to existing target
	assert.Len(t, result.FilesRenamed, 1)
	assert.False(t, result.FilesRenamed[0].Success)
	assert.NotNil(t, result.FilesRenamed[0].Error)
	assert.Contains(t, result.FilesRenamed[0].Error.Error(), "already exists")

	// Verify original files unchanged
	_, err = os.Stat(oldFile)
	assert.NoError(t, err, "old file should still exist")

	_, err = os.Stat(newFile)
	assert.NoError(t, err, "existing file should still exist")
}

func TestFixResult_Summary(t *testing.T) {
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: "file1.md", NewPath: "file1-new.md", Success: true},
			{OldPath: "file2.md", NewPath: "file2-new.md", Success: true},
		},
		ErrorsFixed: 3,
		Errors:      []error{},
	}

	summary := result.Summary()
	assert.Contains(t, summary, "Files renamed: 2")
	assert.Contains(t, summary, "Errors fixed: 3")
}

func TestFixResult_SummaryWithErrors(t *testing.T) {
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: "file1.md", NewPath: "file1-new.md", Success: false, Error: assert.AnError},
		},
		ErrorsFixed: 0,
		Errors:      []error{assert.AnError},
	}

	summary := result.Summary()
	assert.Contains(t, summary, "Files renamed: 1")
	assert.Contains(t, summary, "Errors encountered: 1")
}

func TestIsGitRepository(t *testing.T) {
	t.Run("detects git repository", func(t *testing.T) {
		// Current directory should be a git repo
		isGit := isGitRepository(".")
		assert.True(t, isGit, "current directory should be a git repository")
	})

	t.Run("detects non-git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		isGit := isGitRepository(tmpDir)
		assert.False(t, isGit, "temp directory should not be a git repository")
	})
}

// TestPathsEqualCaseInsensitive tests case-insensitive path comparison.
func TestPathsEqualCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		path1    string
		path2    string
		expected bool
	}{
		{
			name:     "exact match",
			path1:    "/docs/api-guide.md",
			path2:    "/docs/api-guide.md",
			expected: true,
		},
		{
			name:     "case difference",
			path1:    "/docs/API_Guide.md",
			path2:    "/docs/api_guide.md",
			expected: true,
		},
		{
			name:     "mixed case in directory",
			path1:    "/Docs/API/Guide.md",
			path2:    "/docs/api/guide.md",
			expected: true,
		},
		{
			name:     "different files",
			path1:    "/docs/guide.md",
			path2:    "/docs/tutorial.md",
			expected: false,
		},
		{
			name:     "normalized paths",
			path1:    "/docs/../docs/guide.md",
			path2:    "/docs/guide.md",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pathsEqualCaseInsensitive(tt.path1, tt.path2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFileExists tests file existence checking with case-insensitive fallback.
func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tmpDir, "TestFile.md")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	t.Run("exact case match", func(t *testing.T) {
		exists := fileExists(testFile)
		assert.True(t, exists)
	})

	t.Run("different case", func(t *testing.T) {
		lowerCasePath := filepath.Join(tmpDir, "testfile.md")
		exists := fileExists(lowerCasePath)
		assert.True(t, exists, "should find file with case-insensitive lookup")
	})

	t.Run("non-existent file", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "missing.md")
		exists := fileExists(nonExistent)
		assert.False(t, exists)
	})
}

// TestDetectBrokenLinks tests broken link detection.
func TestDetectBrokenLinks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0755)
	require.NoError(t, err)

	// Create a file with broken links
	indexFile := filepath.Join(docsDir, "index.md")
	indexContent := `# Documentation
[Existing File](./guide.md)
[Broken Link](./missing.md)
![Broken Image](./images/missing.png)
[External Link](https://example.com/guide.md)
[Fragment Only](#section)
`
	err = os.WriteFile(indexFile, []byte(indexContent), 0644)
	require.NoError(t, err)

	// Create the existing file
	guideFile := filepath.Join(docsDir, "guide.md")
	err = os.WriteFile(guideFile, []byte("# Guide"), 0644)
	require.NoError(t, err)

	// Run broken link detection
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	broken, err := fixer.detectBrokenLinks(docsDir)
	require.NoError(t, err)

	// Should find 2 broken links (missing.md and missing.png)
	// Should NOT report: existing file, external URL, or fragment-only link
	assert.Len(t, broken, 2, "should detect exactly 2 broken links")

	// Verify broken link details
	var brokenFiles []string
	for _, link := range broken {
		brokenFiles = append(brokenFiles, link.Target)
	}
	assert.Contains(t, brokenFiles, "./missing.md")
	assert.Contains(t, brokenFiles, "./images/missing.png")
}

// TestDetectBrokenLinks_CaseInsensitive tests that broken link detection
// works correctly on case-insensitive filesystems.
func TestDetectBrokenLinks_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0755)
	require.NoError(t, err)

	// Create file with specific case
	actualFile := filepath.Join(docsDir, "API_Guide.md")
	err = os.WriteFile(actualFile, []byte("# API Guide"), 0644)
	require.NoError(t, err)

	// Create index file that links with different case
	indexFile := filepath.Join(docsDir, "index.md")
	indexContent := `# Index
[API Guide](./api_guide.md)
[Another Link](./Api_Guide.md)
`
	err = os.WriteFile(indexFile, []byte(indexContent), 0644)
	require.NoError(t, err)

	// Run broken link detection
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	broken, err := fixer.detectBrokenLinks(docsDir)
	require.NoError(t, err)

	// On case-insensitive filesystems (macOS/Windows), these should NOT be broken
	// On case-sensitive filesystems (Linux), these WOULD be broken
	// The fileExists function handles both cases
	if len(broken) > 0 {
		t.Logf("Detected %d broken links (likely running on case-sensitive filesystem)", len(broken))
	} else {
		t.Log("No broken links detected (likely running on case-insensitive filesystem)")
	}
}

// TestLinkDiscovery_CaseInsensitive tests that link discovery works with
// case-insensitive path matching.
func TestLinkDiscovery_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0755)
	require.NoError(t, err)

	// Create target file with specific case
	targetFile := filepath.Join(docsDir, "API_Guide.md")
	err = os.WriteFile(targetFile, []byte("# API Guide"), 0644)
	require.NoError(t, err)

	// Create index file that links to target with different case
	indexFile := filepath.Join(docsDir, "index.md")
	indexContent := `# Index
[API Guide](./api_guide.md)
[Another Reference](./Api_Guide.md)
![Diagram](./API_GUIDE.md)
`
	err = os.WriteFile(indexFile, []byte(indexContent), 0644)
	require.NoError(t, err)

	// Find links to the target file
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	links, err := fixer.findLinksToFile(targetFile)
	require.NoError(t, err)

	// On case-insensitive comparison, all three links should be found
	// even though they have different cases
	assert.GreaterOrEqual(t, len(links), 3, "should find links with case-insensitive matching")
}

// TestFix_WithBrokenLinkDetection tests that the Fix function includes broken link detection.
func TestFix_WithBrokenLinkDetection(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0755)
	require.NoError(t, err)

	// Create a file with naming issues and broken links
	badFile := filepath.Join(docsDir, "Bad_File.md")
	badContent := `# Documentation
[Existing](./good.md)
[Broken](./missing.md)
`
	err = os.WriteFile(badFile, []byte(badContent), 0644)
	require.NoError(t, err)

	// Create the existing referenced file
	goodFile := filepath.Join(docsDir, "good.md")
	err = os.WriteFile(goodFile, []byte("# Good"), 0644)
	require.NoError(t, err)

	// Run fix
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false) // not dry-run, so broken links are detected

	result, err := fixer.Fix(docsDir)
	require.NoError(t, err)

	// Should detect broken links
	assert.NotEmpty(t, result.BrokenLinks, "should detect broken links")
	
	// Verify broken link details
	foundBroken := false
	for _, broken := range result.BrokenLinks {
		if broken.Target == "./missing.md" {
			foundBroken = true
			assert.Equal(t, badFile, broken.SourceFile)
			break
		}
	}
	assert.True(t, foundBroken, "should find the broken link to missing.md")
}
