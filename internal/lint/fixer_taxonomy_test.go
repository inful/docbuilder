package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFixer_NormalizeTaxonomy_Tags(t *testing.T) { //nolint:dupl // Similar test pattern is acceptable for tags/categories
	tests := []struct {
		name         string
		input        string
		expectedTags []string
		shouldChange bool
	}{
		{
			name: "uppercase tags to lowercase",
			input: `---
title: Test
tags:
  - Golang
  - API
---
# Content`,
			expectedTags: []string{"golang", "api"},
			shouldChange: true,
		},
		{
			name: "tags with spaces to underscores",
			input: `---
title: Test
tags:
  - api reference
  - getting started
---
# Content`,
			expectedTags: []string{"api_reference", "getting_started"},
			shouldChange: true,
		},
		{
			name: "mixed case and spaces",
			input: `---
title: Test
tags:
  - API Reference
  - Getting Started
---
# Content`,
			expectedTags: []string{"api_reference", "getting_started"},
			shouldChange: true,
		},
		{
			name: "already valid tags",
			input: `---
title: Test
tags:
  - golang
  - api-reference
  - v1
---
# Content`,
			expectedTags: []string{"golang", "api-reference", "v1"},
			shouldChange: false,
		},
		{
			name: "no tags field",
			input: `---
title: Test
---
# Content`,
			expectedTags: nil,
			shouldChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.md")
			err := os.WriteFile(filePath, []byte(tt.input), 0o644) //nolint:gosec // Test file permissions
			require.NoError(t, err)

			cfg := &Config{Fix: true, DryRun: false}
			linter := NewLinter(cfg)
			fixer := NewFixer(linter, false, false)

			op := fixer.normalizeTaxonomy(filePath)
			require.NoError(t, op.Error)
			assert.True(t, op.Success, "Operation should succeed")

			// Read the updated file
			updated, err := os.ReadFile(filePath) //nolint:gosec // Test file reading
			require.NoError(t, err)

			if tt.shouldChange {
				// Verify the file was actually changed
				assert.NotEqual(t, tt.input, string(updated), "File should be changed")

				// Verify tags were normalized
				fm, ok := extractFrontmatter(string(updated))
				require.True(t, ok)

				var obj map[string]any
				err = unmarshalYAML([]byte(fm), &obj)
				require.NoError(t, err)

				if tt.expectedTags != nil {
					tags := extractStringArray(obj["tags"])
					assert.Equal(t, tt.expectedTags, tags)
				}
			} else {
				// Verify the file was not changed
				assert.Equal(t, tt.input, string(updated), "File should not be changed")
			}
		})
	}
}

func TestFixer_NormalizeTaxonomy_Categories(t *testing.T) { //nolint:dupl // Similar test pattern is acceptable for tags/categories
	tests := []struct {
		name               string
		input              string
		expectedCategories []string
		shouldChange       bool
	}{
		{
			name: "lowercase categories to capitalized",
			input: `---
title: Test
categories:
  - documentation
  - tutorial
---
# Content`,
			expectedCategories: []string{"Documentation", "Tutorial"},
			shouldChange:       true,
		},
		{
			name: "uppercase categories to capitalized",
			input: `---
title: Test
categories:
  - DOCUMENTATION
  - TUTORIAL
---
# Content`,
			expectedCategories: []string{"Documentation", "Tutorial"},
			shouldChange:       true,
		},
		{
			name: "all caps with spaces",
			input: `---
title: Test
categories:
  - API REFERENCE
  - GETTING STARTED
---
# Content`,
			expectedCategories: []string{"Api reference", "Getting started"},
			shouldChange:       true,
		},
		{
			name: "already valid categories with spaces",
			input: `---
title: Test
categories:
  - API Guide
  - HTTP Protocol
---
# Content`,
			expectedCategories: []string{"API Guide", "HTTP Protocol"},
			shouldChange:       false,
		},
		{
			name: "already valid categories without spaces",
			input: `---
title: Test
categories:
  - Documentation
  - Api-reference
  - V1
---
# Content`,
			expectedCategories: []string{"Documentation", "Api-reference", "V1"},
			shouldChange:       false,
		},
		{
			name: "no categories field",
			input: `---
title: Test
---
# Content`,
			expectedCategories: nil,
			shouldChange:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.md")
			err := os.WriteFile(filePath, []byte(tt.input), 0o644) //nolint:gosec // Test file permissions
			require.NoError(t, err)

			cfg := &Config{Fix: true, DryRun: false}
			linter := NewLinter(cfg)
			fixer := NewFixer(linter, false, false)

			op := fixer.normalizeTaxonomy(filePath)
			require.NoError(t, op.Error)
			assert.True(t, op.Success, "Operation should succeed")

			// Read the updated file
			updated, err := os.ReadFile(filePath) //nolint:gosec // Test file reading
			require.NoError(t, err)

			if tt.shouldChange {
				// Verify the file was actually changed
				assert.NotEqual(t, tt.input, string(updated), "File should be changed")

				// Verify categories were normalized
				fm, ok := extractFrontmatter(string(updated))
				require.True(t, ok)

				var obj map[string]any
				err = unmarshalYAML([]byte(fm), &obj)
				require.NoError(t, err)

				if tt.expectedCategories != nil {
					categories := extractStringArray(obj["categories"])
					assert.Equal(t, tt.expectedCategories, categories)
				}
			} else {
				// Verify the file was not changed
				assert.Equal(t, tt.input, string(updated), "File should not be changed")
			}
		})
	}
}

func TestFixer_NormalizeTaxonomy_DryRun(t *testing.T) {
	input := `---
title: Test
tags:
  - Golang
  - API Reference
categories:
  - documentation
  - TUTORIAL
---
# Content`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.md")
	err := os.WriteFile(filePath, []byte(input), 0o644) //nolint:gosec // Test file permissions
	require.NoError(t, err)

	cfg := &Config{Fix: true, DryRun: true}
	linter := NewLinter(cfg)
	fixer := NewFixer(linter, true, false)

	op := fixer.normalizeTaxonomy(filePath)
	require.NoError(t, op.Error)
	assert.True(t, op.Success, "Dry-run should report success")

	// Read the file - should be unchanged
	updated, err := os.ReadFile(filePath) //nolint:gosec // Test file reading
	require.NoError(t, err)
	assert.Equal(t, input, string(updated), "File should not be modified in dry-run mode")
}

func TestNormalizeTaxonomyInContent(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		shouldChange bool
		checkFunc    func(t *testing.T, output string)
	}{
		{
			name: "normalize tags and categories",
			input: `---
title: Test
tags:
  - Golang
  - API Reference
categories:
  - documentation
  - TUTORIAL
---
# Content`,
			shouldChange: true,
			checkFunc: func(t *testing.T, output string) {
				t.Helper()
				fm, ok := extractFrontmatter(output)
				require.True(t, ok)

				var obj map[string]any
				err := unmarshalYAML([]byte(fm), &obj)
				require.NoError(t, err)

				tags := extractStringArray(obj["tags"])
				assert.Equal(t, []string{"golang", "api_reference"}, tags)

				categories := extractStringArray(obj["categories"])
				assert.Equal(t, []string{"Documentation", "Tutorial"}, categories)
			},
		},
		{
			name: "already valid - no change",
			input: `---
title: Test
tags:
  - golang
  - api
categories:
  - Documentation
  - Tutorial
---
# Content`,
			shouldChange: false,
			checkFunc: func(t *testing.T, output string) {
				t.Helper()
				// Should be unchanged
			},
		},
		{
			name: "no frontmatter",
			input: `# Content without frontmatter

This is just content.`,
			shouldChange: false,
			checkFunc:    nil,
		},
		{
			name: "preserve body content",
			input: `---
title: Test
tags:
  - Golang
---
# Heading

Body content should remain unchanged.

## Subheading

More content here.`,
			shouldChange: true,
			checkFunc: func(t *testing.T, output string) {
				t.Helper()
				// Body should be preserved
				assert.Contains(t, output, "# Heading")
				assert.Contains(t, output, "Body content should remain unchanged")
				assert.Contains(t, output, "## Subheading")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, changed := normalizeTaxonomyInContent(tt.input)
			assert.Equal(t, tt.shouldChange, changed)

			if tt.checkFunc != nil {
				tt.checkFunc(t, output)
			}

			if !tt.shouldChange {
				assert.Equal(t, tt.input, output)
			}
		})
	}
}

// Helper to unmarshal YAML (wrapper for yaml.v3).
func unmarshalYAML(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}
