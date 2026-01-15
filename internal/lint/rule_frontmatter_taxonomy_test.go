package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrontmatterTaxonomyRule_ValidTags(t *testing.T) {
	rule := &FrontmatterTaxonomyRule{}

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "lowercase tags",
			content: `---
title: Test
tags:
  - golang
  - api
  - documentation
---
# Content`,
		},
		{
			name: "tags with hyphens",
			content: `---
title: Test
tags:
  - api-reference
  - getting-started
---
# Content`,
		},
		{
			name: "tags with underscores",
			content: `---
title: Test
tags:
  - api_reference
  - getting_started
---
# Content`,
		},
		{
			name: "tags with numbers",
			content: `---
title: Test
tags:
  - api2
  - v1
  - http2
---
# Content`,
		},
		{
			name: "mixed valid characters",
			content: `---
title: Test
tags:
  - api-v2_reference
  - http2-docs
---
# Content`,
		},
		{
			name: "empty tags array",
			content: `---
title: Test
tags: []
---
# Content`,
		},
		{
			name: "no tags field",
			content: `---
title: Test
---
# Content`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.md")
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			issues, err := rule.Check(filePath)
			require.NoError(t, err)

			// Should have no tag-related errors
			for _, issue := range issues {
				if issue.Severity == SeverityError {
					assert.NotContains(t, issue.Message, "tag", "Should not have tag errors for valid tags")
				}
			}
		})
	}
}

func TestFrontmatterTaxonomyRule_InvalidTags(t *testing.T) {
	rule := &FrontmatterTaxonomyRule{}

	tests := []struct {
		name         string
		content      string
		expectedTags []string // Tags that should be reported as invalid
	}{
		{
			name: "uppercase tags",
			content: `---
title: Test
tags:
  - Golang
  - API
---
# Content`,
			expectedTags: []string{"Golang", "API"},
		},
		{
			name: "mixed case tags",
			content: `---
title: Test
tags:
  - apiReference
  - GettingStarted
---
# Content`,
			expectedTags: []string{"apiReference", "GettingStarted"},
		},
		{
			name: "tags with spaces",
			content: `---
title: Test
tags:
  - api reference
  - getting started
---
# Content`,
			expectedTags: []string{"api reference", "getting started"},
		},
		{
			name: "tags with special characters",
			content: `---
title: Test
tags:
  - api@reference
  - getting.started
---
# Content`,
			expectedTags: []string{"api@reference", "getting.started"},
		},
		{
			name: "mixed valid and invalid",
			content: `---
title: Test
tags:
  - golang
  - API Reference
  - valid-tag
---
# Content`,
			expectedTags: []string{"API Reference"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.md")
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			issues, err := rule.Check(filePath)
			require.NoError(t, err)
			require.NotEmpty(t, issues, "Should have issues for invalid tags")

			// Check that all expected invalid tags are reported
			for _, expectedTag := range tt.expectedTags {
				found := false
				for _, issue := range issues {
					if issue.Severity == SeverityError && 
						(containsStr(issue.Message, expectedTag) || containsStr(issue.Explanation, expectedTag)) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should report invalid tag: %s", expectedTag)
			}
		})
	}
}

func TestFrontmatterTaxonomyRule_ValidCategories(t *testing.T) {
	rule := &FrontmatterTaxonomyRule{}

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "capitalized categories",
			content: `---
title: Test
categories:
  - Documentation
  - Tutorial
  - Guide
---
# Content`,
		},
		{
			name: "categories with hyphens",
			content: `---
title: Test
categories:
  - Api-reference
  - Getting-started
---
# Content`,
		},
		{
			name: "categories with underscores",
			content: `---
title: Test
categories:
  - Api_reference
  - Getting_started
---
# Content`,
		},
		{
			name: "categories with numbers",
			content: `---
title: Test
categories:
  - Api2
  - V1-docs
  - Http2-guide
---
# Content`,
		},
		{
			name: "mixed valid characters",
			content: `---
title: Test
categories:
  - Api-v2_reference
  - Http2-docs
---
# Content`,
		},
		{
			name: "empty categories array",
			content: `---
title: Test
categories: []
---
# Content`,
		},
		{
			name: "no categories field",
			content: `---
title: Test
---
# Content`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.md")
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			issues, err := rule.Check(filePath)
			require.NoError(t, err)

			// Should have no category-related errors
			for _, issue := range issues {
				if issue.Severity == SeverityError {
					assert.NotContains(t, issue.Message, "categor", "Should not have category errors for valid categories")
				}
			}
		})
	}
}

func TestFrontmatterTaxonomyRule_InvalidCategories(t *testing.T) {
	rule := &FrontmatterTaxonomyRule{}

	tests := []struct {
		name               string
		content            string
		expectedCategories []string // Categories that should be reported as invalid
	}{
		{
			name: "all lowercase categories",
			content: `---
title: Test
categories:
  - documentation
  - tutorial
---
# Content`,
			expectedCategories: []string{"documentation", "tutorial"},
		},
		{
			name: "all uppercase categories",
			content: `---
title: Test
categories:
  - DOCUMENTATION
  - TUTORIAL
---
# Content`,
			expectedCategories: []string{"DOCUMENTATION", "TUTORIAL"},
		},
		{
			name: "categories with spaces",
			content: `---
title: Test
categories:
  - API Reference
  - Getting Started
---
# Content`,
			expectedCategories: []string{"API Reference", "Getting Started"},
		},
		{
			name: "categories with special characters",
			content: `---
title: Test
categories:
  - Documentation@Home
  - Guide.Advanced
---
# Content`,
			expectedCategories: []string{"Documentation@Home", "Guide.Advanced"},
		},
		{
			name: "mixed valid and invalid",
			content: `---
title: Test
categories:
  - Documentation
  - getting started
  - Valid-category
---
# Content`,
			expectedCategories: []string{"getting started"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.md")
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			issues, err := rule.Check(filePath)
			require.NoError(t, err)
			require.NotEmpty(t, issues, "Should have issues for invalid categories")

			// Check that all expected invalid categories are reported
			for _, expectedCat := range tt.expectedCategories {
				found := false
				for _, issue := range issues {
					if issue.Severity == SeverityError && 
						(containsStr(issue.Message, expectedCat) || containsStr(issue.Explanation, expectedCat)) {
						found = true
						break
					}
				}
				assert.True(t, found, "Should report invalid category: %s", expectedCat)
			}
		})
	}
}

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
		changed  bool
	}{
		{
			name:     "already valid",
			input:    []string{"golang", "api", "docs"},
			expected: []string{"golang", "api", "docs"},
			changed:  false,
		},
		{
			name:     "uppercase to lowercase",
			input:    []string{"Golang", "API", "Documentation"},
			expected: []string{"golang", "api", "documentation"},
			changed:  true,
		},
		{
			name:     "spaces to underscores",
			input:    []string{"api reference", "getting started"},
			expected: []string{"api_reference", "getting_started"},
			changed:  true,
		},
		{
			name:     "mixed case and spaces",
			input:    []string{"API Reference", "Getting Started"},
			expected: []string{"api_reference", "getting_started"},
			changed:  true,
		},
		{
			name:     "preserve hyphens and underscores",
			input:    []string{"api-reference", "getting_started"},
			expected: []string{"api-reference", "getting_started"},
			changed:  false,
		},
		{
			name:     "preserve numbers",
			input:    []string{"v1", "api2", "http2"},
			expected: []string{"v1", "api2", "http2"},
			changed:  false,
		},
		{
			name:     "empty array",
			input:    []string{},
			expected: []string{},
			changed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := normalizeTags(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.changed, changed)
		})
	}
}

func TestNormalizeCategories(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
		changed  bool
	}{
		{
			name:     "already valid",
			input:    []string{"Documentation", "Tutorial", "Guide"},
			expected: []string{"Documentation", "Tutorial", "Guide"},
			changed:  false,
		},
		{
			name:     "all lowercase to capitalized",
			input:    []string{"documentation", "tutorial"},
			expected: []string{"Documentation", "Tutorial"},
			changed:  true,
		},
		{
			name:     "all uppercase to capitalized",
			input:    []string{"DOCUMENTATION", "TUTORIAL"},
			expected: []string{"Documentation", "Tutorial"},
			changed:  true,
		},
		{
			name:     "spaces to underscores",
			input:    []string{"API Reference", "Getting Started"},
			expected: []string{"Api_reference", "Getting_started"},
			changed:  true,
		},
		{
			name:     "mixed case with spaces",
			input:    []string{"api reference", "GETTING STARTED"},
			expected: []string{"Api_reference", "Getting_started"},
			changed:  true,
		},
		{
			name:     "preserve hyphens and underscores",
			input:    []string{"Api-reference", "Getting_started"},
			expected: []string{"Api-reference", "Getting_started"},
			changed:  false,
		},
		{
			name:     "preserve numbers",
			input:    []string{"V1", "Api2", "Http2"},
			expected: []string{"V1", "Api2", "Http2"},
			changed:  false,
		},
		{
			name:     "capitalize with numbers",
			input:    []string{"v1", "api2"},
			expected: []string{"V1", "Api2"},
			changed:  true,
		},
		{
			name:     "empty array",
			input:    []string{},
			expected: []string{},
			changed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := normalizeCategories(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.changed, changed)
		})
	}
}

// Helper function
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr)
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
