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
