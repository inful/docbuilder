package lint

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update-golden", false, "update golden test files")

// TestGoldenAutoFix_FileRenameWithLinkUpdates tests the complete auto-fix workflow
// including file renaming and link updates, comparing against golden files.
func TestGoldenAutoFix_FileRenameWithLinkUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create a temporary working directory
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "docs")

	// Get absolute path to test data
	testdataDir, err := filepath.Abs(filepath.Join("..", "..", "test", "testdata", "lint", "fix", "links"))
	require.NoError(t, err, "failed to get absolute path to testdata")

	// Copy the "before" test data to working directory
	beforeDir := filepath.Join(testdataDir, "before", "docs")
	err = copyDir(beforeDir, workDir)
	require.NoError(t, err, "failed to copy test data")

	// List files before fix for debugging
	t.Logf("Files before fix:")
	_ = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			t.Logf("  %s", path)
		}
		return nil
	})

	// Run the fixer
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false) // Not dry-run, not force

	result, err := fixer.Fix(workDir)
	require.NoError(t, err, "fix operation should succeed")

	// List files after fix for debugging
	t.Logf("Files after fix:")
	_ = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			t.Logf("  %s", path)
		}
		return nil
	})

	t.Logf("Fix result: %d files renamed, %d links updated", len(result.FilesRenamed), len(result.LinksUpdated))
	for _, rename := range result.FilesRenamed {
		t.Logf("  Rename: %s -> %s (success: %v, error: %v)", rename.OldPath, rename.NewPath, rename.Success, rename.Error)
	}

	// Verify files were renamed
	assert.Greater(t, len(result.FilesRenamed), 0, "should rename files")

	// Verify links were updated
	assert.Greater(t, len(result.LinksUpdated), 0, "should update links")

	// Check if all renames were successful
	for _, rename := range result.FilesRenamed {
		if !rename.Success {
			t.Errorf("Rename failed: %s -> %s: %v", rename.OldPath, rename.NewPath, rename.Error)
		}
	}

	// Compare file contents with expected "after" state
	afterDir := filepath.Join(testdataDir, "after", "docs")
	compareDirectories(t, workDir, afterDir)

	// Verify fix result against golden file
	goldenPath := filepath.Join(testdataDir, "..", "golden", "fix-with-links.golden.json")
	compareFixResultGolden(t, result, goldenPath, *updateGolden)
}

// TestGoldenAutoFix_DryRun tests the dry-run output format.
func TestGoldenAutoFix_DryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create a temporary working directory
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "docs")

	// Get absolute path to test data
	testdataDir, err := filepath.Abs(filepath.Join("..", "..", "test", "testdata", "lint", "fix", "links"))
	require.NoError(t, err, "failed to get absolute path to testdata")

	// Copy the "before" test data
	beforeDir := filepath.Join(testdataDir, "before", "docs")
	err = copyDir(beforeDir, workDir)
	require.NoError(t, err, "failed to copy test data")

	// Run the fixer in dry-run mode
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, true, false) // Dry-run mode

	result, err := fixer.Fix(workDir)
	require.NoError(t, err, "fix operation should succeed")

	// Verify files were NOT actually renamed (dry-run)
	for _, rename := range result.FilesRenamed {
		if rename.Success {
			// In dry-run, original files should still exist
			_, statErr := os.Stat(rename.OldPath)
			assert.NoError(t, statErr, "original file should still exist in dry-run mode")
		}
	}

	// Compare summary output with golden file
	summary := result.Summary()
	goldenPath := filepath.Join(testdataDir, "..", "golden", "fix-dry-run.golden.txt")

	if *updateGolden {
		writeErr := os.WriteFile(goldenPath, []byte(summary), 0o600)
		require.NoError(t, writeErr)
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// #nosec G304 -- test utility reading golden file from testdata
	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file")

	// Normalize paths in summary for comparison (make them relative)
	normalizedSummary := normalizePaths(summary, workDir)
	normalizedExpected := string(expected)

	assert.Equal(t, normalizedExpected, normalizedSummary, "dry-run summary should match golden file")
}

// TestGoldenAutoFix_BrokenLinkDetection tests broken link detection and reporting.
func TestGoldenAutoFix_BrokenLinkDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create test structure with broken links
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	err := os.MkdirAll(docsDir, 0o750)
	require.NoError(t, err)

	// Create a file with broken links
	indexContent := `# Documentation Index

[Existing File](./guide.md)
[Broken Link](./missing.md)
[Another Broken Link](./also-missing.md)
![Broken Image](./images/missing.png)
[External Link](https://example.com/guide.md)
`
	indexFile := filepath.Join(docsDir, "index.md")
	err = os.WriteFile(indexFile, []byte(indexContent), 0o600)
	require.NoError(t, err)

	// Create the existing file
	guideFile := filepath.Join(docsDir, "guide.md")
	err = os.WriteFile(guideFile, []byte("# Guide\n"), 0o600)
	require.NoError(t, err)

	// Run fix (which includes broken link detection)
	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	result, err := fixer.Fix(docsDir)
	require.NoError(t, err)

	// Verify broken links were detected
	assert.NotEmpty(t, result.BrokenLinks, "should detect broken links")

	// Compare with golden file
	goldenDir, err := filepath.Abs(filepath.Join("..", "..", "test", "testdata", "lint", "golden"))
	require.NoError(t, err)
	goldenPath := filepath.Join(goldenDir, "broken-links.golden.json")
	compareBrokenLinksGolden(t, result.BrokenLinks, goldenPath, *updateGolden)
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		// #nosec G304 -- test utility copying files from test directory
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// compareDirectories compares two directory structures and file contents.
// UUIDs in frontmatter are normalized for comparison since they are randomly generated.
func compareDirectories(t *testing.T, actualDir, expectedDir string) {
	t.Helper()

	// Walk expected directory
	err := filepath.Walk(expectedDir, func(expectedPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(expectedDir, expectedPath)
		if err != nil {
			return err
		}

		actualPath := filepath.Join(actualDir, relPath)

		// Read both files
		// #nosec G304 -- test utility reading test output for comparison
		expectedContent, err := os.ReadFile(expectedPath)
		require.NoError(t, err, "failed to read expected file: %s", expectedPath)

		// #nosec G304 -- test utility reading test output for comparison
		actualContent, err := os.ReadFile(actualPath)
		require.NoError(t, err, "failed to read actual file: %s", actualPath)

		// Normalize UUIDs in frontmatter for comparison (UUIDs are randomly generated)
		expectedStr := normalizeUUIDs(string(expectedContent))
		actualStr := normalizeUUIDs(string(actualContent))

		// Compare contents
		assert.Equal(t, expectedStr, actualStr,
			"file content mismatch: %s", relPath)

		return nil
	})

	require.NoError(t, err, "failed to walk expected directory")
}

// normalizeUUIDs replaces UUID values in frontmatter with a placeholder for comparison.
func normalizeUUIDs(content string) string {
	// Replace any UUID (8-4-4-4-12 hex pattern) with PLACEHOLDER_UUID
	// This regex matches standard UUID format (case-insensitive)
	// Using (?i) flag for case-insensitive matching and validating proper UUID structure
	uuidRegex := regexp.MustCompile(`(?i)id:\s*[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`)
	return uuidRegex.ReplaceAllString(content, "id: PLACEHOLDER_UUID")
}

// FixResultForGolden is a normalized version of FixResult for golden file comparison.
type FixResultForGolden struct {
	FilesRenamed  []RenameOperationForGolden `json:"files_renamed"`
	LinksUpdated  []LinkUpdateForGolden      `json:"links_updated"`
	ErrorsFixed   int                        `json:"errors_fixed"`
	WarningsFixed int                        `json:"warnings_fixed"`
	ErrorCount    int                        `json:"error_count"`
}

// RenameOperationForGolden represents a normalized rename operation.
type RenameOperationForGolden struct {
	OldFilename string `json:"old_filename"`
	NewFilename string `json:"new_filename"`
	Success     bool   `json:"success"`
}

// LinkUpdateForGolden represents a normalized link update.
type LinkUpdateForGolden struct {
	SourceFile string `json:"source_file"`
	LineNumber int    `json:"line_number"`
	OldTarget  string `json:"old_target"`
	NewTarget  string `json:"new_target"`
}

// BrokenLinkForGolden represents a normalized broken link.
type BrokenLinkForGolden struct {
	SourceFile string `json:"source_file"`
	LineNumber int    `json:"line_number"`
	Target     string `json:"target"`
	LinkType   string `json:"link_type"`
}

// compareFixResultGolden compares a FixResult against a golden JSON file.
func compareFixResultGolden(t *testing.T, result *FixResult, goldenPath string, update bool) {
	t.Helper()

	// Normalize the result for comparison
	normalized := FixResultForGolden{
		FilesRenamed:  make([]RenameOperationForGolden, len(result.FilesRenamed)),
		LinksUpdated:  make([]LinkUpdateForGolden, len(result.LinksUpdated)),
		ErrorsFixed:   result.ErrorsFixed,
		WarningsFixed: result.WarningsFixed,
		ErrorCount:    len(result.Errors),
	}

	// Normalize file renames (use only filenames, not full paths)
	for i, rename := range result.FilesRenamed {
		normalized.FilesRenamed[i] = RenameOperationForGolden{
			OldFilename: filepath.Base(rename.OldPath),
			NewFilename: filepath.Base(rename.NewPath),
			Success:     rename.Success,
		}
	}

	// Normalize link updates (use only filenames)
	for i, update := range result.LinksUpdated {
		normalized.LinksUpdated[i] = LinkUpdateForGolden{
			SourceFile: filepath.Base(update.SourceFile),
			LineNumber: update.LineNumber,
			OldTarget:  update.OldTarget,
			NewTarget:  update.NewTarget,
		}
	}

	// Sort for consistent comparison (map iteration order is random)
	sort.Slice(normalized.FilesRenamed, func(i, j int) bool {
		return normalized.FilesRenamed[i].OldFilename < normalized.FilesRenamed[j].OldFilename
	})
	sort.Slice(normalized.LinksUpdated, func(i, j int) bool {
		if normalized.LinksUpdated[i].SourceFile != normalized.LinksUpdated[j].SourceFile {
			return normalized.LinksUpdated[i].SourceFile < normalized.LinksUpdated[j].SourceFile
		}
		return normalized.LinksUpdated[i].LineNumber < normalized.LinksUpdated[j].LineNumber
	})

	actualJSON, err := json.MarshalIndent(normalized, "", "  ")
	require.NoError(t, err)

	if update {
		updateErr := os.MkdirAll(filepath.Dir(goldenPath), 0o750)
		require.NoError(t, updateErr)
		updateErr = os.WriteFile(goldenPath, actualJSON, 0o600)
		require.NoError(t, updateErr)
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// #nosec G304 -- test utility reading golden file from testdata
	expectedJSON, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file: %s", goldenPath)

	var expected, actual FixResultForGolden
	err = json.Unmarshal(expectedJSON, &expected)
	require.NoError(t, err)
	err = json.Unmarshal(actualJSON, &actual)
	require.NoError(t, err)

	assert.Equal(t, expected, actual, "fix result should match golden file")
}

// compareBrokenLinksGolden compares broken links against a golden JSON file.
func compareBrokenLinksGolden(t *testing.T, brokenLinks []BrokenLink, goldenPath string, update bool) {
	t.Helper()

	// Normalize broken links
	normalized := make([]BrokenLinkForGolden, len(brokenLinks))
	for i, link := range brokenLinks {
		linkTypeStr := "inline"
		switch link.LinkType {
		case LinkTypeInline:
			linkTypeStr = "inline"
		case LinkTypeImage:
			linkTypeStr = "image"
		case LinkTypeReference:
			linkTypeStr = "reference"
		}

		normalized[i] = BrokenLinkForGolden{
			SourceFile: filepath.Base(link.SourceFile),
			LineNumber: link.LineNumber,
			Target:     link.Target,
			LinkType:   linkTypeStr,
		}
	}

	actualJSON, err := json.MarshalIndent(normalized, "", "  ")
	require.NoError(t, err)

	if update {
		updateErr := os.MkdirAll(filepath.Dir(goldenPath), 0o750)
		require.NoError(t, updateErr)
		updateErr = os.WriteFile(goldenPath, actualJSON, 0o600)
		require.NoError(t, updateErr)
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// #nosec G304 -- test utility reading golden file from testdata
	expectedJSON, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file: %s", goldenPath)

	var expected, actual []BrokenLinkForGolden
	err = json.Unmarshal(expectedJSON, &expected)
	require.NoError(t, err)
	err = json.Unmarshal(actualJSON, &actual)
	require.NoError(t, err)

	assert.Equal(t, expected, actual, "broken links should match golden file")
}

// normalizePaths replaces absolute paths with relative paths for consistent comparison.
func normalizePaths(text, baseDir string) string {
	// This is a simple implementation; could be enhanced with regex
	// For now, we'll just remove the base directory path
	return text // TODO: Implement path normalization if needed
}
