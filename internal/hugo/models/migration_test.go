package models

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationHelper_ConvertLegacyPatch(t *testing.T) {
	helper := NewMigrationHelper()
	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    map[string]any
		validate func(*testing.T, *FrontMatterPatch)
	}{
		{
			name:  "nil input",
			input: nil,
			validate: func(t *testing.T, patch *FrontMatterPatch) {
				assert.True(t, patch.IsEmpty())
			},
		},
		{
			name: "complete legacy patch",
			input: map[string]any{
				"title":                "Test Title",
				"date":                 testTime,
				"draft":                true,
				"description":          "Test Description",
				"repository":           "test-repo",
				"forge":                "github",
				"section":              "docs",
				"edit_url":             "https://github.com/test/edit",
				"weight":               10,
				"layout":               "single",
				"type":                 "page",
				"tags":                 []string{"tag1", "tag2"},
				"categories":           []string{"cat1", "cat2"},
				"keywords":             []string{"key1", "key2"},
				"merge_mode":           "replace",
				"array_merge_strategy": "append",
				"custom_field":         "custom_value",
			},
			validate: func(t *testing.T, patch *FrontMatterPatch) {
				assert.Equal(t, "Test Title", *patch.Title)
				assert.True(t, testTime.Equal(*patch.Date))
				assert.Equal(t, true, *patch.Draft)
				assert.Equal(t, "Test Description", *patch.Description)
				assert.Equal(t, "test-repo", *patch.Repository)
				assert.Equal(t, "github", *patch.Forge)
				assert.Equal(t, "docs", *patch.Section)
				assert.Equal(t, "https://github.com/test/edit", *patch.EditURL)
				assert.Equal(t, 10, *patch.Weight)
				assert.Equal(t, "single", *patch.Layout)
				assert.Equal(t, "page", *patch.Type)
				assert.Equal(t, []string{"tag1", "tag2"}, *patch.Tags)
				assert.Equal(t, []string{"cat1", "cat2"}, *patch.Categories)
				assert.Equal(t, []string{"key1", "key2"}, *patch.Keywords)
				assert.Equal(t, MergeModeReplace, patch.MergeMode)
				assert.Equal(t, ArrayMergeStrategyAppend, patch.ArrayMergeStrategy)
				assert.Equal(t, "custom_value", patch.Custom["custom_field"])
			},
		},
		{
			name: "date as string",
			input: map[string]any{
				"title": "Test",
				"date":  "2023-12-25T10:30:00Z",
			},
			validate: func(t *testing.T, patch *FrontMatterPatch) {
				assert.Equal(t, "Test", *patch.Title)
				assert.True(t, testTime.Equal(*patch.Date))
			},
		},
		{
			name: "interface arrays",
			input: map[string]any{
				"tags":       []interface{}{"tag1", "tag2"},
				"categories": []interface{}{"cat1", "cat2"},
				"keywords":   []interface{}{"key1", "key2"},
			},
			validate: func(t *testing.T, patch *FrontMatterPatch) {
				assert.Equal(t, []string{"tag1", "tag2"}, *patch.Tags)
				assert.Equal(t, []string{"cat1", "cat2"}, *patch.Categories)
				assert.Equal(t, []string{"key1", "key2"}, *patch.Keywords)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := helper.ConvertLegacyPatch(tt.input)
			require.NoError(t, err)
			tt.validate(t, result)
		})
	}
}

func TestMigrationHelper_CreateBasePatch(t *testing.T) {
	helper := NewMigrationHelper()

	patch := helper.CreateBasePatch("Test Title", "test-repo", "github", "docs")

	assert.Equal(t, "Test Title", *patch.Title)
	assert.Equal(t, "test-repo", *patch.Repository)
	assert.Equal(t, "github", *patch.Forge)
	assert.Equal(t, "docs", *patch.Section)
	assert.False(t, patch.Date.IsZero())
	assert.Equal(t, MergeModeSetIfMissing, patch.MergeMode)
}

func TestMigrationHelper_ApplyPatchSequence(t *testing.T) {
	helper := NewMigrationHelper()

	base := &FrontMatter{
		Title:      "Original",
		Repository: "test-repo",
		Tags:       []string{"original_tag"},
	}

	patch1 := NewFrontMatterPatch().
		SetDescription("Added description").
		WithMergeMode(MergeModeSetIfMissing)

	patch2 := NewFrontMatterPatch().
		SetTitle("Updated Title").
		SetTags([]string{"patch2_tag"}).
		WithMergeMode(MergeModeDeep).
		WithArrayMergeStrategy(ArrayMergeStrategyUnion)

	result, err := helper.ApplyPatchSequence(base, patch1, patch2)
	require.NoError(t, err)

	assert.Equal(t, "Updated Title", result.Title)                       // Overridden by patch2
	assert.Equal(t, "Added description", result.Description)             // Set by patch1
	assert.Equal(t, "test-repo", result.Repository)                      // From base
	assert.Equal(t, []string{"original_tag", "patch2_tag"}, result.Tags) // Union merge with original
}

func TestMigrationHelper_ApplyPatchSequence_NilBase(t *testing.T) {
	helper := NewMigrationHelper()
	patch := NewFrontMatterPatch().SetTitle("Test")

	result, err := helper.ApplyPatchSequence(nil, patch)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "base front matter cannot be nil")
}

func TestMigrationHelper_ApplyPatchSequence_NilPatch(t *testing.T) {
	helper := NewMigrationHelper()
	base := &FrontMatter{Title: "Test"}

	result, err := helper.ApplyPatchSequence(base, nil)
	require.NoError(t, err)
	assert.Equal(t, "Test", result.Title)
}

func TestMigrationHelper_ValidateFrontMatterConfig(t *testing.T) {
	helper := NewMigrationHelper()

	tests := []struct {
		name             string
		fm               *FrontMatter
		expectedWarnings int
		expectedMsgs     []string
	}{
		{
			name: "valid front matter",
			fm: &FrontMatter{
				Title:      "Test",
				Date:       time.Now(),
				Repository: "test-repo",
			},
			expectedWarnings: 0,
		},
		{
			name: "missing required fields",
			fm: &FrontMatter{
				Custom: map[string]interface{}{},
			},
			expectedWarnings: 3,
			expectedMsgs:     []string{"title is empty", "date is not set", "repository is not set"},
		},
		{
			name: "complex custom fields",
			fm: &FrontMatter{
				Title:      "Test",
				Date:       time.Now(),
				Repository: "test-repo",
				Custom: map[string]interface{}{
					"": "empty key",
					"complex": map[string]interface{}{
						"nested1": 1, "nested2": 2, "nested3": 3, "nested4": 4, "nested5": 5,
						"nested6": 6, "nested7": 7, "nested8": 8, "nested9": 9, "nested10": 10,
						"nested11": 11, // This makes it >10 items
					},
					"large_array": make([]interface{}, 25), // >20 items
				},
			},
			expectedWarnings: 3,
			expectedMsgs:     []string{"empty custom field key", "complex nested structure", "large array"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := helper.ValidateFrontMatterConfig(tt.fm)
			assert.Len(t, warnings, tt.expectedWarnings)

			for _, msg := range tt.expectedMsgs {
				found := false
				for _, warning := range warnings {
					if strings.Contains(warning, msg) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected warning containing '%s' not found in: %v", msg, warnings)
			}
		})
	}
}

func TestMigrationHelper_convertToStringArray(t *testing.T) {
	helper := NewMigrationHelper()

	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "string array",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "interface array",
			input:    []interface{}{"a", "b", 123},
			expected: []string{"a", "b", "123"},
		},
		{
			name:     "single string",
			input:    "single",
			expected: []string{"single"},
		},
		{
			name:     "unsupported type",
			input:    123,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.convertToStringArray(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMigrationHelper_parseMergeMode(t *testing.T) {
	helper := NewMigrationHelper()

	tests := []struct {
		input    string
		expected MergeMode
		wantErr  bool
	}{
		{"deep", MergeModeDeep, false},
		{"replace", MergeModeReplace, false},
		{"set_if_missing", MergeModeSetIfMissing, false},
		{"invalid", MergeModeDeep, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := helper.parseMergeMode(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMigrationHelper_parseArrayMergeStrategy(t *testing.T) {
	helper := NewMigrationHelper()

	tests := []struct {
		input    string
		expected ArrayMergeStrategy
		wantErr  bool
	}{
		{"append", ArrayMergeStrategyAppend, false},
		{"union", ArrayMergeStrategyUnion, false},
		{"replace", ArrayMergeStrategyReplace, false},
		{"invalid", ArrayMergeStrategyUnion, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := helper.parseArrayMergeStrategy(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// LegacyCompatibilityAdapter-related tests removed after refactor to typed-only APIs.
