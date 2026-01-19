package pipeline

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildBaseFrontMatter(t *testing.T) {
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		doc      *Document
		expected map[string]any
	}{
		{
			name: "basic transformation",
			doc: &Document{
				Name:        "getting-started",
				FrontMatter: make(map[string]any),
				CommitDate:  fixedTime,
			},
			expected: map[string]any{
				"title": "Getting Started",
				"type":  "docs",
				"date":  "2023-01-01T12:00:00+00:00",
			},
		},
		{
			name: "snake_case title",
			doc: &Document{
				Name:        "user_guide",
				FrontMatter: make(map[string]any),
				CommitDate:  fixedTime,
			},
			expected: map[string]any{
				"title": "User Guide",
				"type":  "docs",
				"date":  "2023-01-01T12:00:00+00:00",
			},
		},
		{
			name: "existing title preserved",
			doc: &Document{
				Name: "getting-started",
				FrontMatter: map[string]any{
					"title": "Existing Title",
				},
				CommitDate: fixedTime,
			},
			expected: map[string]any{
				"title": "Existing Title",
				"type":  "docs",
				"date":  "2023-01-01T12:00:00+00:00",
			},
		},
		{
			name: "index file title fallback omitted (handled by later transform)",
			doc: &Document{
				Name:        "index",
				IsIndex:     true,
				FrontMatter: make(map[string]any),
				CommitDate:  fixedTime,
			},
			expected: map[string]any{
				"type": "docs",
				"date": "2023-01-01T12:00:00+00:00",
			},
		},
		{
			name: "empty name fallback",
			doc: &Document{
				Name:        "",
				FrontMatter: make(map[string]any),
				CommitDate:  fixedTime,
			},
			expected: map[string]any{
				"title": "Untitled",
				"type":  "docs",
				"date":  "2023-01-01T12:00:00+00:00",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildBaseFrontMatter(tt.doc)
			require.NoError(t, err)
			for k, v := range tt.expected {
				assert.Equal(t, v, tt.doc.FrontMatter[k], "mismatch for field %s", k)
			}
		})
	}
}

func TestFormatTitle(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"getting-started", "Getting Started"},
		{"user_guide", "User Guide"},
		{"multi-part-name_with_mix", "Multi Part Name With Mix"},
		{"single", "Single"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatTitle(tt.name))
		})
	}
}
