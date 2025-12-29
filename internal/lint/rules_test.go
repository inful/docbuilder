package lint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilenameRule_ValidFiles(t *testing.T) {
	rule := &FilenameRule{}

	validFiles := []string{
		"readme.md",
		"getting-started.md",
		"api_reference.md",
		"image.png",
		"diagram.svg",
		"photo-2024.jpg",
		"config-file-yaml.md", // Valid - yaml is part of filename, not extension
		"123-numbers.md",
		"file.with.dots.md",
	}

	for _, file := range validFiles {
		t.Run(file, func(t *testing.T) {
			issues, err := rule.Check(file)
			require.NoError(t, err)

			// Should have no errors or warnings
			hasProblems := false
			for _, issue := range issues {
				if issue.Severity == SeverityError || issue.Severity == SeverityWarning {
					hasProblems = true
					t.Errorf("Expected no issues for valid file %s, got: %s", file, issue.Message)
				}
			}
			assert.False(t, hasProblems, "Valid file should not have errors or warnings")
		})
	}
}

func TestFilenameRule_WhitelistedDoubleExtensions(t *testing.T) {
	rule := &FilenameRule{}

	whitelistedFiles := []string{
		"architecture.drawio.png",
		"diagram.drawio.svg",
		"System-Overview.drawio.png", // Mixed case but whitelisted
		"FLOWCHART.DRAWIO.SVG",       // All caps but whitelisted
	}

	for _, file := range whitelistedFiles {
		t.Run(file, func(t *testing.T) {
			issues, err := rule.Check(file)
			require.NoError(t, err)

			// Should have at least one info issue about whitelist
			hasInfo := false
			for _, issue := range issues {
				if issue.Severity == SeverityInfo {
					hasInfo = true
					assert.Contains(t, issue.Message, "Whitelisted")
				}
				// May also have uppercase errors for non-lowercase whitelisted files
				if issue.Severity == SeverityError && hasUppercase(file) {
					assert.Contains(t, issue.Message, "uppercase")
				}
			}
			assert.True(t, hasInfo, "Whitelisted file should have info issue")
		})
	}
}

func TestFilenameRule_InvalidDoubleExtensions(t *testing.T) {
	rule := &FilenameRule{}

	invalidFiles := []string{
		"readme.md.backup",
		"config.yaml.old",
		"image.png.tmp",
		"document.markdown.bak",
	}

	for _, file := range invalidFiles {
		t.Run(file, func(t *testing.T) {
			issues, err := rule.Check(file)
			require.NoError(t, err)
			require.NotEmpty(t, issues, "Should have issues for invalid double extension")

			hasDoubleExtError := false
			for _, issue := range issues {
				if issue.Severity == SeverityError && contains(issue.Message, "double extension") {
					hasDoubleExtError = true
				}
			}
			assert.True(t, hasDoubleExtError, "Should have error about invalid double extension")
		})
	}
}

func TestFilenameRule_UppercaseLetters(t *testing.T) {
	rule := &FilenameRule{}

	tests := []struct {
		name     string
		filename string
		wantFix  string
	}{
		{
			name:     "all uppercase",
			filename: "README.MD",
			wantFix:  "readme.md",
		},
		{
			name:     "mixed case",
			filename: "GettingStarted.md",
			wantFix:  "gettingstarted.md",
		},
		{
			name:     "camel case",
			filename: "apiReference.md",
			wantFix:  "apireference.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := rule.Check(tt.filename)
			require.NoError(t, err)
			require.NotEmpty(t, issues, "Should have issues for uppercase")

			hasUppercaseError := false
			for _, issue := range issues {
				if issue.Severity == SeverityError && contains(issue.Message, "uppercase") {
					hasUppercaseError = true
					assert.Contains(t, issue.Fix, tt.wantFix, "Fix should suggest correct filename")
				}
			}
			assert.True(t, hasUppercaseError, "Should have error about uppercase letters")
		})
	}
}

func TestFilenameRule_Spaces(t *testing.T) {
	rule := &FilenameRule{}

	tests := []struct {
		name     string
		filename string
		wantFix  string
	}{
		{
			name:     "single space",
			filename: "my document.md",
			wantFix:  "my-document.md",
		},
		{
			name:     "multiple spaces",
			filename: "getting  started  guide.md",
			wantFix:  "getting-started-guide.md",
		},
		{
			name:     "leading space",
			filename: " readme.md",
			wantFix:  "readme.md",
		},
		{
			name:     "trailing space",
			filename: "readme .md",
			wantFix:  "readme.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := rule.Check(tt.filename)
			require.NoError(t, err)
			require.NotEmpty(t, issues, "Should have issues for spaces")

			hasSpaceError := false
			for _, issue := range issues {
				if issue.Severity == SeverityError && contains(issue.Message, "space") {
					hasSpaceError = true
					assert.Contains(t, issue.Fix, tt.wantFix, "Fix should suggest correct filename")
				}
			}
			assert.True(t, hasSpaceError, "Should have error about spaces")
		})
	}
}

func TestFilenameRule_SpecialCharacters(t *testing.T) {
	rule := &FilenameRule{}

	tests := []struct {
		name        string
		filename    string
		wantFix     string
		wantInvalid []string
	}{
		{
			name:        "parentheses",
			filename:    "file(1).md",
			wantFix:     "file1.md",
			wantInvalid: []string{"(", ")"},
		},
		{
			name:        "brackets",
			filename:    "file[draft].md",
			wantFix:     "filedraft.md",
			wantInvalid: []string{"[", "]"},
		},
		{
			name:        "ampersand",
			filename:    "api&reference.md",
			wantFix:     "apireference.md",
			wantInvalid: []string{"&"},
		},
		{
			name:        "special chars",
			filename:    "file@#$%.md",
			wantFix:     "file.md",
			wantInvalid: []string{"@", "#", "$", "%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := rule.Check(tt.filename)
			require.NoError(t, err)
			require.NotEmpty(t, issues, "Should have issues for special characters")

			hasSpecialCharError := false
			for _, issue := range issues {
				if issue.Severity == SeverityError && contains(issue.Message, "special character") {
					hasSpecialCharError = true
					assert.Contains(t, issue.Fix, tt.wantFix, "Fix should suggest correct filename")
					// Check that invalid characters are mentioned
					for _, char := range tt.wantInvalid {
						assert.Contains(t, issue.Message, char, "Should mention invalid character")
					}
				}
			}
			assert.True(t, hasSpecialCharError, "Should have error about special characters")
		})
	}
}

func TestFilenameRule_LeadingTrailingSeparators(t *testing.T) {
	rule := &FilenameRule{}

	tests := []struct {
		name     string
		filename string
		wantFix  string
	}{
		{
			name:     "leading hyphen",
			filename: "-readme.md",
			wantFix:  "readme.md",
		},
		{
			name:     "trailing hyphen",
			filename: "readme-.md",
			wantFix:  "readme.md",
		},
		{
			name:     "leading underscore",
			filename: "_config.md",
			wantFix:  "config.md",
		},
		{
			name:     "trailing underscore",
			filename: "config_.md",
			wantFix:  "config.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := rule.Check(tt.filename)
			require.NoError(t, err)
			require.NotEmpty(t, issues, "Should have issues for leading/trailing separators")

			hasSeparatorError := false
			for _, issue := range issues {
				if issue.Severity == SeverityError && contains(issue.Message, "leading or trailing") {
					hasSeparatorError = true
					assert.Contains(t, issue.Fix, tt.wantFix, "Fix should suggest correct filename")
				}
			}
			assert.True(t, hasSeparatorError, "Should have error about leading/trailing separators")
		})
	}
}

func TestSuggestFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"README.MD", "readme.md"},
		{"Getting Started.md", "getting-started.md"},
		{"API Reference Guide.md", "api-reference-guide.md"},
		{"file(1).md", "file1.md"},
		{"doc@#$%.md", "doc.md"},
		{"-readme-.md", "readme.md"},
		{"config__file.md", "config__file.md"}, // Double underscore is allowed
		{"my---file.md", "my-file.md"},         // Multiple hyphens collapsed
		{"file  with   spaces.md", "file-with-spaces.md"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SuggestFilename(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectDefaultPath(t *testing.T) {
	// Note: This test depends on the actual filesystem.
	// In the DocBuilder project, we should have a docs/ directory.
	path, found := DetectDefaultPath()

	// Should detect "docs" if it exists
	if found {
		assert.Equal(t, "docs", path, "Should detect docs directory")
	} else {
		// Fallback to current directory
		assert.Equal(t, ".", path, "Should fallback to current directory")
	}
}

func TestResult_Counts(t *testing.T) {
	result := &Result{
		Issues: []Issue{
			{Severity: SeverityError, Message: "error1"},
			{Severity: SeverityError, Message: "error2"},
			{Severity: SeverityWarning, Message: "warning1"},
			{Severity: SeverityInfo, Message: "info1"},
		},
		FilesTotal: 10,
	}

	assert.Equal(t, 2, result.ErrorCount())
	assert.Equal(t, 1, result.WarningCount())
	assert.True(t, result.HasErrors())
	assert.True(t, result.HasWarnings())
}

func TestResult_NoIssues(t *testing.T) {
	result := &Result{
		Issues:     []Issue{},
		FilesTotal: 5,
	}

	assert.Equal(t, 0, result.ErrorCount())
	assert.Equal(t, 0, result.WarningCount())
	assert.False(t, result.HasErrors())
	assert.False(t, result.HasWarnings())
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
