package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFrontMatter(t *testing.T) {
	fm := NewFrontMatter()

	assert.NotNil(t, fm)
	assert.NotNil(t, fm.Custom)
	assert.False(t, fm.Date.IsZero())
}

func TestFrontMatter_FromMap(t *testing.T) {
	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    map[string]any
		expected *FrontMatter
		wantErr  bool
	}{
		{
			name: "complete front matter",
			input: map[string]any{
				"title":        "Test Title",
				"date":         testTime,
				"draft":        true,
				"description":  "Test Description",
				"repository":   "test-repo",
				"forge":        "github",
				"section":      "docs",
				"edit_url":     "https://github.com/test/edit",
				"weight":       10,
				"layout":       "single",
				"type":         "page",
				"tags":         []string{"tag1", "tag2"},
				"categories":   []string{"cat1", "cat2"},
				"keywords":     []string{"key1", "key2"},
				"custom_field": "custom_value",
			},
			expected: &FrontMatter{
				Title:       "Test Title",
				Date:        testTime,
				Draft:       true,
				Description: "Test Description",
				Repository:  "test-repo",
				Forge:       "github",
				Section:     "docs",
				EditURL:     "https://github.com/test/edit",
				Weight:      10,
				Layout:      "single",
				Type:        "page",
				Tags:        []string{"tag1", "tag2"},
				Categories:  []string{"cat1", "cat2"},
				Keywords:    []string{"key1", "key2"},
				Custom: map[string]interface{}{
					"custom_field": "custom_value",
				},
			},
			wantErr: false,
		},
		{
			name: "date as string RFC3339",
			input: map[string]any{
				"title": "Test",
				"date":  "2023-12-25T10:30:00Z",
			},
			expected: &FrontMatter{
				Title:  "Test",
				Date:   testTime,
				Custom: map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "interface arrays",
			input: map[string]any{
				"title":      "Test",
				"tags":       []interface{}{"tag1", "tag2"},
				"categories": []interface{}{"cat1", "cat2"},
				"keywords":   []interface{}{"key1", "key2"},
			},
			expected: &FrontMatter{
				Title:      "Test",
				Tags:       []string{"tag1", "tag2"},
				Categories: []string{"cat1", "cat2"},
				Keywords:   []string{"key1", "key2"},
				Custom:     map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name:  "empty map",
			input: map[string]any{},
			expected: &FrontMatter{
				Custom: map[string]interface{}{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FromMap(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Title, result.Title)
			assert.Equal(t, tt.expected.Draft, result.Draft)
			assert.Equal(t, tt.expected.Description, result.Description)
			assert.Equal(t, tt.expected.Repository, result.Repository)
			assert.Equal(t, tt.expected.Forge, result.Forge)
			assert.Equal(t, tt.expected.Section, result.Section)
			assert.Equal(t, tt.expected.EditURL, result.EditURL)
			assert.Equal(t, tt.expected.Weight, result.Weight)
			assert.Equal(t, tt.expected.Layout, result.Layout)
			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.Tags, result.Tags)
			assert.Equal(t, tt.expected.Categories, result.Categories)
			assert.Equal(t, tt.expected.Keywords, result.Keywords)
			assert.Equal(t, tt.expected.Custom, result.Custom)

			if !tt.expected.Date.IsZero() {
				assert.True(t, tt.expected.Date.Equal(result.Date))
			}
		})
	}
}

func TestFrontMatter_ToMap(t *testing.T) {
	testTime := time.Date(2023, 12, 25, 10, 30, 0, 0, time.UTC)

	fm := &FrontMatter{
		Title:       "Test Title",
		Date:        testTime,
		Draft:       true,
		Description: "Test Description",
		Repository:  "test-repo",
		Forge:       "github",
		Section:     "docs",
		EditURL:     "https://github.com/test/edit",
		Weight:      10,
		Layout:      "single",
		Type:        "page",
		Tags:        []string{"tag1", "tag2"},
		Categories:  []string{"cat1", "cat2"},
		Keywords:    []string{"key1", "key2"},
		Custom: map[string]interface{}{
			"custom_field": "custom_value",
		},
	}

	result := fm.ToMap()

	assert.Equal(t, "Test Title", result["title"])
	assert.Equal(t, "2023-12-25T10:30:00+00:00", result["date"])
	assert.Equal(t, true, result["draft"])
	assert.Equal(t, "Test Description", result["description"])
	assert.Equal(t, "test-repo", result["repository"])
	assert.Equal(t, "github", result["forge"])
	assert.Equal(t, "docs", result["section"])
	assert.Equal(t, "https://github.com/test/edit", result["edit_url"])
	assert.Equal(t, 10, result["weight"])
	assert.Equal(t, "single", result["layout"])
	assert.Equal(t, "page", result["type"])
	assert.Equal(t, []string{"tag1", "tag2"}, result["tags"])
	assert.Equal(t, []string{"cat1", "cat2"}, result["categories"])
	assert.Equal(t, []string{"key1", "key2"}, result["keywords"])
	assert.Equal(t, "custom_value", result["custom_field"])
}

func TestFrontMatter_Clone(t *testing.T) {
	original := &FrontMatter{
		Title:      "Original",
		Tags:       []string{"tag1", "tag2"},
		Categories: []string{"cat1"},
		Custom: map[string]interface{}{
			"custom": "value",
		},
	}

	clone := original.Clone()

	// Verify deep copy
	assert.Equal(t, original.Title, clone.Title)
	assert.Equal(t, original.Tags, clone.Tags)
	assert.Equal(t, original.Categories, clone.Categories)
	assert.Equal(t, original.Custom, clone.Custom)

	// Verify independence
	clone.Title = "Modified"
	clone.Tags[0] = "modified_tag"
	clone.Custom["custom"] = "modified"

	assert.Equal(t, "Original", original.Title)
	assert.Equal(t, "tag1", original.Tags[0])
	assert.Equal(t, "value", original.Custom["custom"])
}

func TestFrontMatter_CustomFields(t *testing.T) {
	fm := NewFrontMatter()

	// Test SetCustom
	fm.SetCustom("test_key", "test_value")

	// Test GetCustom
	value, exists := fm.GetCustom("test_key")
	assert.True(t, exists)
	assert.Equal(t, "test_value", value)

	// Test GetCustomString
	str, exists := fm.GetCustomString("test_key")
	assert.True(t, exists)
	assert.Equal(t, "test_value", str)

	// Test GetCustomInt
	fm.SetCustom("int_key", 42)
	intVal, exists := fm.GetCustomInt("int_key")
	assert.True(t, exists)
	assert.Equal(t, 42, intVal)

	// Test non-existent key
	_, exists = fm.GetCustom("non_existent")
	assert.False(t, exists)
}

func TestFrontMatter_AddMethods(t *testing.T) {
	fm := NewFrontMatter()

	// Test AddTag
	fm.AddTag("tag1")
	fm.AddTag("tag2")
	fm.AddTag("tag1") // Duplicate
	assert.Equal(t, []string{"tag1", "tag2"}, fm.Tags)

	// Test AddCategory
	fm.AddCategory("cat1")
	fm.AddCategory("cat2")
	fm.AddCategory("cat1") // Duplicate
	assert.Equal(t, []string{"cat1", "cat2"}, fm.Categories)

	// Test AddKeyword
	fm.AddKeyword("key1")
	fm.AddKeyword("key2")
	fm.AddKeyword("key1") // Duplicate
	assert.Equal(t, []string{"key1", "key2"}, fm.Keywords)
}

func TestFrontMatter_Validate(t *testing.T) {
	tests := []struct {
		name    string
		fm      *FrontMatter
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid front matter",
			fm: &FrontMatter{
				Title:      "Test",
				Date:       time.Now(),
				Repository: "test-repo",
			},
			wantErr: false,
		},
		{
			name: "missing title",
			fm: &FrontMatter{
				Date:       time.Now(),
				Repository: "test-repo",
			},
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name: "missing date",
			fm: &FrontMatter{
				Title:      "Test",
				Repository: "test-repo",
			},
			wantErr: true,
			errMsg:  "date is required",
		},
		{
			name: "missing repository",
			fm: &FrontMatter{
				Title: "Test",
				Date:  time.Now(),
			},
			wantErr: true,
			errMsg:  "repository is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fm.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
