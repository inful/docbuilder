package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFixResult_PreviewChanges tests the preview output for user confirmation.
func TestFixResult_PreviewChanges(t *testing.T) {
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: "/tmp/docs/API_Guide.md", NewPath: "/tmp/docs/api-guide.md", Success: true},
			{OldPath: "/tmp/docs/User Manual.md", NewPath: "/tmp/docs/user-manual.md", Success: true},
		},
		LinksUpdated: []LinkUpdate{
			{SourceFile: "/tmp/docs/index.md", LineNumber: 10, OldTarget: "./API_Guide.md", NewTarget: "./api-guide.md"},
			{SourceFile: "/tmp/docs/index.md", LineNumber: 15, OldTarget: "./User Manual.md", NewTarget: "./user-manual.md"},
			{SourceFile: "/tmp/docs/guide.md", LineNumber: 5, OldTarget: "../API_Guide.md", NewTarget: "../api-guide.md"},
		},
	}

	preview := result.PreviewChanges()

	// Verify preview contains expected sections
	assert.Contains(t, preview, "FILE RENAMES:", "should show file renames section")
	assert.Contains(t, preview, "API_Guide.md → api-guide.md", "should show first rename")
	assert.Contains(t, preview, "User Manual.md → user-manual.md", "should show second rename")

	assert.Contains(t, preview, "LINKS TO UPDATE:", "should show links section")
	assert.Contains(t, preview, "index.md (2 links)", "should group links by file")
	assert.Contains(t, preview, "guide.md (1 link)", "should show second file")

	assert.Contains(t, preview, "SUMMARY:", "should show summary")
	assert.Contains(t, preview, "2 files will be renamed", "should count renamed files")
	assert.Contains(t, preview, "3 links will be updated", "should count updated links")
}

// TestFixResult_DetailedPreview tests the detailed dry-run preview output.
func TestFixResult_DetailedPreview(t *testing.T) {
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: "/tmp/docs/API_Guide.md", NewPath: "/tmp/docs/api-guide.md", Success: true},
		},
		LinksUpdated: []LinkUpdate{
			{SourceFile: "/tmp/docs/index.md", LineNumber: 10, OldTarget: "./API_Guide.md", NewTarget: "./api-guide.md"},
		},
		BrokenLinks: []BrokenLink{
			{SourceFile: "/tmp/docs/index.md", LineNumber: 20, Target: "./missing.md", LinkType: LinkTypeInline},
		},
	}

	preview := result.DetailedPreview()

	// Verify detailed preview shows full paths and line-by-line changes
	assert.Contains(t, preview, "[File Renames]", "should show file renames header")
	assert.Contains(t, preview, "/tmp/docs/API_Guide.md", "should show full old path")
	assert.Contains(t, preview, "/tmp/docs/api-guide.md", "should show full new path")

	assert.Contains(t, preview, "[Link Updates]", "should show link updates header")
	assert.Contains(t, preview, "/tmp/docs/index.md:10", "should show file and line number")
	assert.Contains(t, preview, "Before: ./API_Guide.md", "should show before state")
	assert.Contains(t, preview, "After:  ./api-guide.md", "should show after state")

	assert.Contains(t, preview, "[Broken Links Detected]", "should show broken links header")
	assert.Contains(t, preview, "./missing.md (file not found)", "should show broken link target")
}

// TestFixResult_HasChanges tests the HasChanges method.
func TestFixResult_HasChanges(t *testing.T) {
	tests := []struct {
		name     string
		result   *FixResult
		expected bool
	}{
		{
			name: "has file renames",
			result: &FixResult{
				FilesRenamed: []RenameOperation{{OldPath: "a.md", NewPath: "b.md"}},
			},
			expected: true,
		},
		{
			name: "has link updates",
			result: &FixResult{
				LinksUpdated: []LinkUpdate{{SourceFile: "a.md", OldTarget: "b.md", NewTarget: "c.md"}},
			},
			expected: true,
		},
		{
			name: "has both",
			result: &FixResult{
				FilesRenamed: []RenameOperation{{OldPath: "a.md", NewPath: "b.md"}},
				LinksUpdated: []LinkUpdate{{SourceFile: "a.md", OldTarget: "b.md", NewTarget: "c.md"}},
			},
			expected: true,
		},
		{
			name:     "no changes",
			result:   &FixResult{},
			expected: false,
		},
		{
			name: "only broken links (not changes)",
			result: &FixResult{
				BrokenLinks: []BrokenLink{{SourceFile: "a.md", Target: "missing.md"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.HasChanges())
		})
	}
}

// TestFixResult_CountAffectedFiles tests counting unique files that will be modified.
func TestFixResult_CountAffectedFiles(t *testing.T) {
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: "/tmp/docs/API_Guide.md", NewPath: "/tmp/docs/api-guide.md"},
			{OldPath: "/tmp/docs/User Manual.md", NewPath: "/tmp/docs/user-manual.md"},
		},
		LinksUpdated: []LinkUpdate{
			{SourceFile: "/tmp/docs/index.md", LineNumber: 10},
			{SourceFile: "/tmp/docs/index.md", LineNumber: 15}, // Same file, counted once
			{SourceFile: "/tmp/docs/guide.md", LineNumber: 5},
		},
	}

	// Should count: 2 renamed files + 2 unique files with link updates = 4
	assert.Equal(t, 4, result.CountAffectedFiles())
}

// TestFixer_CreateBackup tests backup creation functionality.
func TestFixer_CreateBackup(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0o755)
	require.NoError(t, err)

	// Create test files
	file1 := filepath.Join(docsDir, "api.md")
	file2 := filepath.Join(docsDir, "guide.md")
	err = os.WriteFile(file1, []byte("API content"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("Guide content"), 0o644)
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

	content1, err := os.ReadFile(backupFile1)
	assert.NoError(t, err, "should backup first file")
	assert.Equal(t, "API content", string(content1))

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
	err := os.MkdirAll(docsDir, 0o755)
	require.NoError(t, err)

	// Create test file with naming issue
	badFile := filepath.Join(docsDir, "API Guide.md")
	err = os.WriteFile(badFile, []byte("# API Guide\n"), 0o644)
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

// TestPreviewPluraliz	ation tests the pluralize function output in messages.
func TestPreviewPluralization(t *testing.T) {
	tests := []struct {
		name         string
		filesRenamed int
		linksUpdated int
		expectFiles  string
		expectLinks  string
	}{
		{
			name:         "singular",
			filesRenamed: 1,
			linksUpdated: 1,
			expectFiles:  "1 file will be renamed",
			expectLinks:  "1 link will be updated",
		},
		{
			name:         "plural",
			filesRenamed: 2,
			linksUpdated: 5,
			expectFiles:  "2 files will be renamed",
			expectLinks:  "5 links will be updated",
		},
		{
			name:         "zero (plural)",
			filesRenamed: 0,
			linksUpdated: 0,
			expectFiles:  "0 files will be renamed",
			expectLinks:  "0 links will be updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &FixResult{
				FilesRenamed: make([]RenameOperation, tt.filesRenamed),
				LinksUpdated: make([]LinkUpdate, tt.linksUpdated),
			}

			preview := result.PreviewChanges()
			assert.Contains(t, preview, tt.expectFiles)
			assert.Contains(t, preview, tt.expectLinks)
		})
	}
}

// TestBackupPreservesDirectoryStructure tests that backup preserves relative paths.
func TestBackupPreservesDirectoryStructure(t *testing.T) {
	// Create nested directory structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	apiDir := filepath.Join(docsDir, "api")
	err := os.MkdirAll(apiDir, 0o755)
	require.NoError(t, err)

	// Create files in different directories
	rootFile := filepath.Join(docsDir, "index.md")
	apiFile := filepath.Join(apiDir, "guide.md")

	err = os.WriteFile(rootFile, []byte("root content"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(apiFile, []byte("api content"), 0o644)
	require.NoError(t, err)

	// Create result
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: rootFile},
		},
		LinksUpdated: []LinkUpdate{
			{SourceFile: apiFile},
		},
	}

	// Create backup
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	backupDir, err := fixer.CreateBackup(result, docsDir)
	require.NoError(t, err)

	// Verify directory structure is preserved
	backupRoot := filepath.Join(backupDir, "index.md")
	backupAPI := filepath.Join(backupDir, "api", "guide.md")

	content, err := os.ReadFile(backupRoot)
	assert.NoError(t, err)
	assert.Equal(t, "root content", string(content))

	content, err = os.ReadFile(backupAPI)
	assert.NoError(t, err)
	assert.Equal(t, "api content", string(content))
}
