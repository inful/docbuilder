package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFrontMatterPatch(t *testing.T) {
	patch := NewFrontMatterPatch()

	assert.NotNil(t, patch)
	assert.NotNil(t, patch.Custom)
	assert.Equal(t, MergeModeDeep, patch.MergeMode)
	assert.Equal(t, ArrayMergeStrategyUnion, patch.ArrayMergeStrategy)
	assert.True(t, patch.IsEmpty())
}

func TestFrontMatterPatch_SetMethods(t *testing.T) {
	patch := NewFrontMatterPatch()
	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	// Test fluent interface
	result := patch.
		SetTitle("Test Title").
		SetDate(testTime).
		SetDraft(true).
		SetDescription("Test Description").
		SetRepository("test-repo").
		SetForge("github").
		SetSection("docs").
		SetEditURL("https://github.com/test/edit").
		SetWeight(10).
		SetLayout("single").
		SetType("page").
		SetTags([]string{"tag1", "tag2"}).
		SetCategories([]string{"cat1", "cat2"}).
		SetKeywords([]string{"key1", "key2"}).
		SetCustom("custom_field", "custom_value").
		WithMergeMode(MergeModeReplace).
		WithArrayMergeStrategy(ArrayMergeStrategyAppend)

	assert.Same(t, patch, result) // Fluent interface returns same instance

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
	assert.Equal(t, "custom_value", patch.Custom["custom_field"])
	assert.Equal(t, MergeModeReplace, patch.MergeMode)
	assert.Equal(t, ArrayMergeStrategyAppend, patch.ArrayMergeStrategy)

	assert.False(t, patch.IsEmpty())
}

func TestFrontMatterPatch_Apply_Replace(t *testing.T) {
	original := &FrontMatter{
		Title:      "Original Title",
		Tags:       []string{"old_tag"},
		Categories: []string{"old_cat"},
		Custom: map[string]interface{}{
			"old_field": "old_value",
		},
	}

	patch := NewFrontMatterPatch().
		SetTitle("New Title").
		SetTags([]string{"new_tag"}).
		SetCustom("new_field", "new_value").
		WithMergeMode(MergeModeReplace)

	result, err := patch.Apply(original)
	require.NoError(t, err)

	// Verify replacement
	assert.Equal(t, "New Title", result.Title)
	assert.Equal(t, []string{"new_tag"}, result.Tags)
	assert.Equal(t, "new_value", result.Custom["new_field"])
	assert.Equal(t, "old_value", result.Custom["old_field"]) // Custom fields are additive

	// Verify original unchanged
	assert.Equal(t, "Original Title", original.Title)
	assert.Equal(t, []string{"old_tag"}, original.Tags)
}

func TestFrontMatterPatch_Apply_SetIfMissing(t *testing.T) {
	original := &FrontMatter{
		Title:       "Existing Title",
		Description: "", // Empty, should be set
		Tags:        []string{"existing_tag"},
		Categories:  nil, // Nil, should be set
		Custom: map[string]interface{}{
			"existing_field": "existing_value",
		},
	}

	patch := NewFrontMatterPatch().
		SetTitle("New Title").                    // Should not override
		SetDescription("New Desc").               // Should set (empty)
		SetTags([]string{"new_tag"}).             // Should not override (has items)
		SetCategories([]string{"new_cat"}).       // Should set (nil/empty)
		SetCustom("new_field", "new_value").      // Should set (missing)
		SetCustom("existing_field", "new_value"). // Should not override
		WithMergeMode(MergeModeSetIfMissing)

	result, err := patch.Apply(original)
	require.NoError(t, err)

	assert.Equal(t, "Existing Title", result.Title)                    // Not overridden
	assert.Equal(t, "New Desc", result.Description)                    // Set because empty
	assert.Equal(t, []string{"existing_tag"}, result.Tags)             // Not overridden
	assert.Equal(t, []string{"new_cat"}, result.Categories)            // Set because empty
	assert.Equal(t, "existing_value", result.Custom["existing_field"]) // Not overridden
	assert.Equal(t, "new_value", result.Custom["new_field"])           // Set because missing
}

func TestFrontMatterPatch_Apply_Deep(t *testing.T) {
	original := &FrontMatter{
		Title: "Original Title",
		Tags:  []string{"tag1", "tag2"},
		Custom: map[string]interface{}{
			"existing_field": "existing_value",
		},
	}

	tests := []struct {
		name               string
		arrayMergeStrategy ArrayMergeStrategy
		patchTags          []string
		expectedTags       []string
	}{
		{
			name:               "union strategy",
			arrayMergeStrategy: ArrayMergeStrategyUnion,
			patchTags:          []string{"tag2", "tag3"}, // tag2 is duplicate
			expectedTags:       []string{"tag1", "tag2", "tag3"},
		},
		{
			name:               "append strategy",
			arrayMergeStrategy: ArrayMergeStrategyAppend,
			patchTags:          []string{"tag2", "tag3"}, // tag2 will be duplicated
			expectedTags:       []string{"tag1", "tag2", "tag2", "tag3"},
		},
		{
			name:               "replace strategy",
			arrayMergeStrategy: ArrayMergeStrategyReplace,
			patchTags:          []string{"tag3", "tag4"},
			expectedTags:       []string{"tag3", "tag4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch := NewFrontMatterPatch().
				SetTitle("New Title").
				SetTags(tt.patchTags).
				SetCustom("new_field", "new_value").
				WithMergeMode(MergeModeDeep).
				WithArrayMergeStrategy(tt.arrayMergeStrategy)

			result, err := patch.Apply(original)
			require.NoError(t, err)

			assert.Equal(t, "New Title", result.Title)
			assert.Equal(t, tt.expectedTags, result.Tags)
			assert.Equal(t, "existing_value", result.Custom["existing_field"])
			assert.Equal(t, "new_value", result.Custom["new_field"])
		})
	}
}

func TestFrontMatterPatch_Apply_NilInput(t *testing.T) {
	patch := NewFrontMatterPatch().SetTitle("Test")

	result, err := patch.Apply(nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot apply patch to nil front matter")
}

func TestFrontMatterPatch_mergeStringArray(t *testing.T) {
	patch := NewFrontMatterPatch()

	tests := []struct {
		name     string
		strategy ArrayMergeStrategy
		existing []string
		new      []string
		expected []string
	}{
		{
			name:     "replace",
			strategy: ArrayMergeStrategyReplace,
			existing: []string{"a", "b"},
			new:      []string{"c", "d"},
			expected: []string{"c", "d"},
		},
		{
			name:     "append",
			strategy: ArrayMergeStrategyAppend,
			existing: []string{"a", "b"},
			new:      []string{"b", "c"},
			expected: []string{"a", "b", "b", "c"},
		},
		{
			name:     "union",
			strategy: ArrayMergeStrategyUnion,
			existing: []string{"a", "b"},
			new:      []string{"b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "union with empty existing",
			strategy: ArrayMergeStrategyUnion,
			existing: []string{},
			new:      []string{"a", "b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "union with empty new",
			strategy: ArrayMergeStrategyUnion,
			existing: []string{"a", "b"},
			new:      []string{},
			expected: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch.ArrayMergeStrategy = tt.strategy
			result := patch.mergeStringArray(tt.existing, tt.new)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFrontMatterPatch_ToMap(t *testing.T) {
	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	patch := NewFrontMatterPatch().
		SetTitle("Test Title").
		SetDate(testTime).
		SetDraft(true).
		SetTags([]string{"tag1", "tag2"}).
		SetCustom("custom_field", "custom_value")

	result := patch.ToMap()

	assert.Equal(t, "Test Title", result["title"])
	assert.Equal(t, "2023-12-25T10:30:00+00:00", result["date"])
	assert.Equal(t, true, result["draft"])
	assert.Equal(t, []string{"tag1", "tag2"}, result["tags"])
	assert.Equal(t, "custom_value", result["custom_field"])
}

func TestFrontMatterPatch_IsEmpty(t *testing.T) {
	patch := NewFrontMatterPatch()
	assert.True(t, patch.IsEmpty())

	patch.SetTitle("Test")
	assert.False(t, patch.IsEmpty())

	patch = NewFrontMatterPatch()
	patch.SetCustom("key", "value")
	assert.False(t, patch.IsEmpty())
}

func TestFrontMatterPatch_String(t *testing.T) {
	patch := NewFrontMatterPatch().
		SetTitle("Test Title").
		SetRepository("test-repo").
		SetSection("docs").
		SetCustom("key1", "value1").
		SetCustom("key2", "value2")

	result := patch.String()

	assert.Contains(t, result, "FrontMatterPatch{")
	assert.Contains(t, result, `title="Test Title"`)
	assert.Contains(t, result, `repository="test-repo"`)
	assert.Contains(t, result, `section="docs"`)
	assert.Contains(t, result, "custom=2_fields")
}

func TestMergeMode_String(t *testing.T) {
	tests := []struct {
		mode     MergeMode
		expected string
	}{
		{MergeModeDeep, "deep"},
		{MergeModeReplace, "replace"},
		{MergeModeSetIfMissing, "set_if_missing"},
		{MergeMode(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestArrayMergeStrategy_String(t *testing.T) {
	tests := []struct {
		strategy ArrayMergeStrategy
		expected string
	}{
		{ArrayMergeStrategyAppend, "append"},
		{ArrayMergeStrategyUnion, "union"},
		{ArrayMergeStrategyReplace, "replace"},
		{ArrayMergeStrategy(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.strategy.String())
		})
	}
}
