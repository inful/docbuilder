package lint

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

// TestFixResult_Summary tests the Summary method.
func TestFixResult_Summary(t *testing.T) {
	result := &FixResult{
		FilesRenamed: []RenameOperation{
			{OldPath: "file1.md", NewPath: "file1-new.md", Success: true},
			{OldPath: "file2.md", NewPath: "file2-new.md", Success: true},
		},
		ErrorsFixed:   3,
		WarningsFixed: 2,
		Errors:        []error{},
	}

	summary := result.Summary()
	assert.Contains(t, summary, "Files renamed: 2")
	assert.Contains(t, summary, "Errors fixed: 3")
	assert.Contains(t, summary, "Warnings fixed: 2")
}

// TestFixResult_SummaryWithErrors tests the Summary method with errors.
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

// TestPreviewPluralization tests the pluralize function output in messages.
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
