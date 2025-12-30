package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/lint"
)

func TestLintGolden_ValidCorrectFilenames(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	testDir := "../../test/testdata/lint/valid/correct-filenames"
	goldenPath := "../../test/testdata/lint/golden/correct-filenames.golden.json"

	linter := lint.NewLinter(&lint.Config{Format: "text"})
	result, err := linter.LintPath(testDir)
	require.NoError(t, err, "lint operation failed")

	verifyLintResult(t, result, goldenPath, *updateGolden)
}

func TestLintGolden_ValidWhitelistedExtensions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	testDir := "../../test/testdata/lint/valid/whitelisted-extensions"
	goldenPath := "../../test/testdata/lint/golden/whitelisted-extensions.golden.json"

	linter := lint.NewLinter(&lint.Config{Format: "text"})
	result, err := linter.LintPath(testDir)
	require.NoError(t, err, "lint operation failed")

	verifyLintResult(t, result, goldenPath, *updateGolden)
}

func TestLintGolden_InvalidMixedCase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	testDir := "../../test/testdata/lint/invalid/mixed-case"
	goldenPath := "../../test/testdata/lint/golden/mixed-case.golden.json"

	linter := lint.NewLinter(&lint.Config{Format: "text"})
	result, err := linter.LintPath(testDir)
	require.NoError(t, err, "lint operation failed")

	verifyLintResult(t, result, goldenPath, *updateGolden)
}

func TestLintGolden_InvalidSpaces(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	testDir := "../../test/testdata/lint/invalid/spaces"
	goldenPath := "../../test/testdata/lint/golden/spaces.golden.json"

	linter := lint.NewLinter(&lint.Config{Format: "text"})
	result, err := linter.LintPath(testDir)
	require.NoError(t, err, "lint operation failed")

	verifyLintResult(t, result, goldenPath, *updateGolden)
}

func TestLintGolden_InvalidSpecialChars(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	testDir := "../../test/testdata/lint/invalid/special-chars"
	goldenPath := "../../test/testdata/lint/golden/special-chars.golden.json"

	linter := lint.NewLinter(&lint.Config{Format: "text"})
	result, err := linter.LintPath(testDir)
	require.NoError(t, err, "lint operation failed")

	verifyLintResult(t, result, goldenPath, *updateGolden)
}

func TestLintGolden_InvalidDoubleExtensions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	testDir := "../../test/testdata/lint/invalid/invalid-double-ext"
	goldenPath := "../../test/testdata/lint/golden/invalid-double-ext.golden.json"

	linter := lint.NewLinter(&lint.Config{Format: "text"})
	result, err := linter.LintPath(testDir)
	require.NoError(t, err, "lint operation failed")

	verifyLintResult(t, result, goldenPath, *updateGolden)
}

// verifyLintResult compares a lint result against a golden file.
func verifyLintResult(t *testing.T, result *lint.Result, goldenPath string, updateGolden bool) {
	t.Helper()

	// Normalize the result for comparison (remove system-specific paths)
	normalized := normalizeLintResult(result)

	if updateGolden {
		data, err := json.MarshalIndent(normalized, "", "  ")
		require.NoError(t, err, "failed to marshal result")

		err = os.MkdirAll(filepath.Dir(goldenPath), 0o755)
		require.NoError(t, err, "failed to create golden directory")

		err = os.WriteFile(goldenPath, data, 0o644)
		require.NoError(t, err, "failed to write golden file")

		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// Read and compare with golden file
	goldenData, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file: %s", goldenPath)

	var expected normalizedResult
	err = json.Unmarshal(goldenData, &expected)
	require.NoError(t, err, "failed to unmarshal golden file")

	// Compare JSON representations for better error messages
	actualJSON, err := json.MarshalIndent(normalized, "", "  ")
	require.NoError(t, err)

	expectedJSON, err := json.MarshalIndent(expected, "", "  ")
	require.NoError(t, err)

	assert.JSONEq(t, string(expectedJSON), string(actualJSON),
		"lint result doesn't match golden file: %s\nRun with -update-golden to update", goldenPath)
}

// normalizedResult represents a lint result with normalized paths for golden testing.
type normalizedResult struct {
	Issues     []normalizedIssue `json:"issues"`
	FileCount  int               `json:"file_count"`
	ErrorCount int               `json:"error_count"`
	WarnCount  int               `json:"warn_count"`
	InfoCount  int               `json:"info_count"`
	HasErrors  bool              `json:"has_errors"`
}

type normalizedIssue struct {
	File        string `json:"file"`
	Severity    string `json:"severity"`
	Rule        string `json:"rule"`
	Message     string `json:"message"`
	Explanation string `json:"explanation,omitempty"`
	Fix         string `json:"fix,omitempty"`
	Line        int    `json:"line,omitempty"`
}

// normalizeLintResult converts a Result to a normalized version for golden testing.
func normalizeLintResult(result *lint.Result) normalizedResult {
	normalized := normalizedResult{
		Issues:     make([]normalizedIssue, 0, len(result.Issues)),
		FileCount:  result.FilesTotal,
		ErrorCount: result.ErrorCount(),
		WarnCount:  result.WarningCount(),
		InfoCount:  len(result.Issues) - result.ErrorCount() - result.WarningCount(),
		HasErrors:  result.HasErrors(),
	}

	for _, issue := range result.Issues {
		// Normalize file path to be relative and use forward slashes
		normalizedFile := normalizeFilePath(issue.FilePath)

		normalized.Issues = append(normalized.Issues, normalizedIssue{
			File:        normalizedFile,
			Severity:    issue.Severity.String(),
			Rule:        issue.Rule,
			Message:     issue.Message,
			Explanation: issue.Explanation,
			Fix:         issue.Fix,
			Line:        issue.Line,
		})
	}

	return normalized
}

// normalizeFilePath removes system-specific path components and converts to forward slashes.
func normalizeFilePath(path string) string {
	// Convert to forward slashes
	path = filepath.ToSlash(path)

	// Remove absolute path prefix, keeping only relative path from test directory
	// e.g., "/path/to/test/testdata/lint/invalid/spaces/User Manual.md"
	//    -> "invalid/spaces/User Manual.md"
	marker := "testdata/lint/"
	if idx := strings.LastIndex(path, marker); idx >= 0 {
		// Keep the part after "testdata/lint/"
		return path[idx+len(marker):]
	}

	// Fallback: just return the basename
	return filepath.Base(path)
}
