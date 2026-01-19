package pipeline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectPermalink(t *testing.T) {
	baseURL := "https://docs.example.com/"
	transform := injectPermalink(baseURL)

	t.Run("Injects permalink when UID and alias match", func(t *testing.T) {
		doc := &Document{
			Extension: ".md",
			FrontMatter: map[string]any{
				"title": "My Page",
				"uid":   "12345",
				"aliases": []any{
					"/_uid/12345/",
				},
			},
			Content: "# My Page\n\nContent here.",
		}

		_, err := transform(doc)
		require.NoError(t, err)

		assert.Contains(t, doc.Content, "https://docs.example.com/_uid/12345/")
		assert.Contains(t, doc.Content, "[my-page]")
		assert.Contains(t, doc.Content, "{{% badge style=\"note\" title=\"permalink\" %}}")
	})

	t.Run("Does not inject when already present", func(t *testing.T) {
		doc := &Document{
			Extension: ".md",
			FrontMatter: map[string]any{
				"uid": "12345",
				"aliases": []any{
					"/_uid/12345/",
				},
			},
			Content: "# Title\n\n[permalink](https://docs.example.com/_uid/12345/)",
		}

		_, err := transform(doc)
		require.NoError(t, err)

		// Count occurrences
		count := strings.Count(doc.Content, "https://docs.example.com/_uid/12345/")
		assert.Equal(t, 1, count)
	})

	t.Run("Does not inject when UID missing", func(t *testing.T) {
		doc := &Document{
			Extension: ".md",
			FrontMatter: map[string]any{
				"aliases": []any{
					"/_uid/12345/",
				},
			},
			Content: "# Title",
		}

		_, err := transform(doc)
		require.NoError(t, err)
		assert.NotContains(t, doc.Content, "permalink-badge")
	})

	t.Run("Does not inject when alias missing", func(t *testing.T) {
		doc := &Document{
			Extension: ".md",
			FrontMatter: map[string]any{
				"uid": "12345",
			},
			Content: "# Title",
		}

		_, err := transform(doc)
		require.NoError(t, err)
		assert.NotContains(t, doc.Content, "permalink-badge")
	})

	t.Run("Skips non-markdown files", func(t *testing.T) {
		doc := &Document{
			Extension: ".png",
			FrontMatter: map[string]any{
				"uid":     "12345",
				"aliases": []any{"/_uid/12345/"},
			},
			Content: "Binary content",
		}

		_, err := transform(doc)
		require.NoError(t, err)
		assert.NotContains(t, doc.Content, "permalink-badge")
	})
}
