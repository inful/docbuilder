package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFixer_CanFixFilename tests the canFixFilename method.
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

// TestFixer_DryRun tests dry-run mode behavior.
func TestFixer_DryRun(t *testing.T) {
	// Create a temporary directory with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "API_Guide.md")

	err := os.WriteFile(testFile, []byte("# API Guide"), 0o600)
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

// TestFixer_RenameFile tests basic file renaming.
func TestFixer_RenameFile(t *testing.T) {
	// Create a temporary directory with a test file
	tmpDir := t.TempDir()
	oldFile := filepath.Join(tmpDir, "API_Guide.md")
	expectedNewFile := filepath.Join(tmpDir, "api_guide.md")

	err := os.WriteFile(oldFile, []byte("# API Guide"), 0o600)
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
	assert.Equal(t, 3, result.ErrorsFixed) // rename + frontmatter uid + frontmatter fingerprint

	// #nosec G304 -- test reads a temp file path under t.TempDir().
	updatedBytes, readErr := os.ReadFile(expectedNewFile)
	require.NoError(t, readErr)
	uid, hasUID := extractUIDFromFrontmatter(string(updatedBytes))
	require.True(t, hasUID)
	_, parseErr := uuid.Parse(uid)
	require.NoError(t, parseErr)
	rule := &FrontmatterFingerprintRule{}
	issues, err := rule.Check(expectedNewFile)
	require.NoError(t, err)
	require.Empty(t, issues)
}

// TestFixer_RenameMultipleFiles tests renaming multiple files.
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
		err := os.WriteFile(oldPath, []byte("# Test"), 0o600)
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

// TestFixer_ErrorWhenTargetExists tests error handling when target file exists.
func TestFixer_ErrorWhenTargetExists(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create both old and new files
	oldFile := filepath.Join(tmpDir, "API_Guide.md")
	newFile := filepath.Join(tmpDir, "api_guide.md")

	err := os.WriteFile(oldFile, []byte("# API Guide"), 0o600)
	require.NoError(t, err)

	err = os.WriteFile(newFile, []byte("# Existing"), 0o600)
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

// TestFixer_CreateBackup tests backup creation functionality.
func TestFixer_CreateBackup(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0o750)
	require.NoError(t, err)

	// Create test files
	file1 := filepath.Join(docsDir, "api.md")
	file2 := filepath.Join(docsDir, "guide.md")
	err = os.WriteFile(file1, []byte("API content"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("Guide content"), 0o600)
	require.NoError(t, err)

	// Create result with changes
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: file1, NewPath: filepath.Join(docsDir, "api-guide.md")},
		},
		LinksUpdated: []LinkUpdate{
			{SourceFile: file2, LineNumber: 5},
		},
	}

	// Create fixer and backup
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	backupDir, err := fixer.CreateBackup(result, docsDir)
	require.NoError(t, err)
	assert.NotEmpty(t, backupDir, "should return backup directory path")

	// Verify backup directory exists
	_, err = os.Stat(backupDir)
	assert.NoError(t, err, "backup directory should exist")

	// Verify backup directory name format
	assert.Contains(t, filepath.Base(backupDir), ".docbuilder-backup-")

	// Verify backed up files exist
	backupFile1 := filepath.Join(backupDir, "api.md")
	backupFile2 := filepath.Join(backupDir, "guide.md")

	// #nosec G304 -- test utility reading backup files from test directory
	content1, err := os.ReadFile(backupFile1)
	assert.NoError(t, err, "should backup first file")
	assert.Equal(t, "API content", string(content1))

	// #nosec G304 -- test utility reading backup files from test directory
	content2, err := os.ReadFile(backupFile2)
	assert.NoError(t, err, "should backup second file")
	assert.Equal(t, "Guide content", string(content2))
}

// TestFixer_CreateBackup_DryRun tests that dry-run mode skips backup.
func TestFixer_CreateBackup_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: filepath.Join(tmpDir, "api.md"), NewPath: filepath.Join(tmpDir, "api-guide.md")},
		},
	}

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, true, false) // dry-run mode

	backupDir, err := fixer.CreateBackup(result, tmpDir)
	require.NoError(t, err)
	assert.Empty(t, backupDir, "dry-run mode should not create backup")
}

// TestFixer_WithAutoConfirm tests the auto-confirm flag.
func TestFixer_WithAutoConfirm(t *testing.T) {
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	// Initial state
	assert.False(t, fixer.autoConfirm)

	// Set auto-confirm
	fixer = fixer.WithAutoConfirm(true)
	assert.True(t, fixer.autoConfirm)

	// Verify it returns the fixer for chaining
	fixer2 := fixer.WithAutoConfirm(false)
	assert.Same(t, fixer, fixer2, "should return same instance for chaining")
}

// TestFixer_ConfirmChanges_AutoConfirm tests that auto-confirm skips prompts.
func TestFixer_ConfirmChanges_AutoConfirm(t *testing.T) {
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: "/tmp/api.md", NewPath: "/tmp/api-guide.md"},
		},
	}

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)

	// Should auto-confirm without prompting
	confirmed, err := fixer.ConfirmChanges(result)
	require.NoError(t, err)
	assert.True(t, confirmed, "auto-confirm should return true")
}

// TestFixer_ConfirmChanges_DryRun tests that dry-run mode skips prompts.
func TestFixer_ConfirmChanges_DryRun(t *testing.T) {
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: "/tmp/api.md", NewPath: "/tmp/api-guide.md"},
		},
	}

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, true, false) // dry-run mode

	// Should auto-confirm in dry-run
	confirmed, err := fixer.ConfirmChanges(result)
	require.NoError(t, err)
	assert.True(t, confirmed, "dry-run should auto-confirm")
}

// TestFixer_ConfirmChanges_NoChanges tests that no prompt shown when no changes.
func TestFixer_ConfirmChanges_NoChanges(t *testing.T) {
	result := &FixResult{} // No changes

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	// Should return true without prompting when there are no changes
	confirmed, err := fixer.ConfirmChanges(result)
	require.NoError(t, err)
	assert.True(t, confirmed, "should auto-confirm when no changes")
}

// TestFixer_FixWithConfirmation_Integration tests the full confirmation workflow.
func TestFixer_FixWithConfirmation_Integration(t *testing.T) {
	// Create test structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0o750)
	require.NoError(t, err)

	// Create test file with naming issue
	badFile := filepath.Join(docsDir, "API Guide.md")
	err = os.WriteFile(badFile, []byte("# API Guide\n"), 0o600)
	require.NoError(t, err)

	// Use auto-confirm to avoid interactive prompt in test
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)

	// Run with confirmation (should auto-confirm due to flag)
	result, err := fixer.FixWithConfirmation(docsDir)
	require.NoError(t, err)

	// Verify fix was applied
	assert.Greater(t, len(result.FilesRenamed), 0, "should have renamed files")
	assert.True(t, result.FilesRenamed[0].Success, "rename should succeed")

	// Verify new file exists
	expectedPath := filepath.Join(docsDir, "api-guide.md")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "renamed file should exist")

	// Verify old file is gone
	_, err = os.Stat(badFile)
	assert.Error(t, err, "old file should not exist")

	// Verify backup was created
	backupFiles, err := filepath.Glob(filepath.Join(docsDir, ".docbuilder-backup-*"))
	require.NoError(t, err)
	assert.Greater(t, len(backupFiles), 0, "backup directory should be created")
}
