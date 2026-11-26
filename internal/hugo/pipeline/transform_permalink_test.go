package pipeline

import (
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// copyAttribute matches the attribute that holds the text a copy
// button copies.
var copyAttribute = regexp.MustCompile(`data-docbuilder-copy="([^"]*)"`)

// copyPayloads returns the texts the copy buttons of the content copy,
// as the browser reads them.
func copyPayloads(t *testing.T, content string) []string {
	t.Helper()
	matches := copyAttribute.FindAllStringSubmatch(content, -1)
	payloads := make([]string, 0, len(matches))
	for _, m := range matches {
		payloads = append(payloads, html.UnescapeString(m[1]))
	}
	return payloads
}

func TestInjectPermalink(t *testing.T) {
	baseURL := "https://docs.example.com/"
	transform := injectPermalink(baseURL)

	t.Run("Injects permalink when UID and alias match", func(t *testing.T) {
		doc := &Document{
			Extension: markdownExtension,
			FrontMatter: map[string]any{
				"title": "My Page",
				"uid":   "12345",
				testPermalinkAliasesKey: []any{
					testPermalinkAliasUID,
				},
			},
			Content: "# My Page\n\nContent here.",
		}

		_, err := transform(doc)
		require.NoError(t, err)

		assert.Contains(t, doc.Content, "https://docs.example.com/_uid/12345/")
		assert.Contains(t, doc.Content, permalinkMarker)

		// The two buttons copy the markdown link and the HTML link.
		payloads := copyPayloads(t, doc.Content)
		require.Len(t, payloads, 2)
		assert.Equal(t, "[My Page](https://docs.example.com/_uid/12345/)", payloads[0])
		assert.Equal(t, `<a href="https://docs.example.com/_uid/12345/">My Page</a>`, payloads[1])

		// The link itself points at the stable alias of the page.
		assert.Contains(t, doc.Content,
			`<a href="https://docs.example.com/_uid/12345/">My Page</a>`)
	})

	t.Run("Escapes the title of the page", func(t *testing.T) {
		doc := &Document{
			Extension: markdownExtension,
			FrontMatter: map[string]any{
				"title": "A [bracketed] title",
				"uid":   "12345",
				testPermalinkAliasesKey: []any{
					testPermalinkAliasUID,
				},
			},
			Content: "# Title",
		}

		_, err := transform(doc)
		require.NoError(t, err)

		payloads := copyPayloads(t, doc.Content)
		require.Len(t, payloads, 2)
		assert.Equal(t,
			`[A \[bracketed\] title](https://docs.example.com/_uid/12345/)`,
			payloads[0],
			"the brackets of the title would break the markdown link")
		assert.Equal(t,
			`<a href="https://docs.example.com/_uid/12345/">A [bracketed] title</a>`,
			payloads[1])
	})

	t.Run("Does not inject when already present", func(t *testing.T) {
		doc := &Document{
			Extension: markdownExtension,
			FrontMatter: map[string]any{
				"uid": "12345",
				testPermalinkAliasesKey: []any{
					testPermalinkAliasUID,
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

	t.Run("Does not inject a second permalink", func(t *testing.T) {
		doc := &Document{
			Extension: markdownExtension,
			FrontMatter: map[string]any{
				"title": "My Page",
				"uid":   "12345",
				testPermalinkAliasesKey: []any{
					testPermalinkAliasUID,
				},
			},
			Content: "# My Page\n\n" + permalinkBlock("My Page", "https://docs.example.com/_uid/12345/"),
		}

		_, err := transform(doc)
		require.NoError(t, err)

		assert.Len(t, copyPayloads(t, doc.Content), 2,
			"the document already carries a permalink")
	})

	t.Run("Does not inject when UID missing", func(t *testing.T) {
		doc := &Document{
			Extension: markdownExtension,
			FrontMatter: map[string]any{
				testPermalinkAliasesKey: []any{
					testPermalinkAliasUID,
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
			Extension: markdownExtension,
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
