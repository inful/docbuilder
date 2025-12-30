package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeShortcodesInCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "code block with no shortcodes",
			content:  "Here is a code block:\n\n```\nfoo()\nbar()\n```\n\nEnd of doc.",
			expected: "Here is a code block:\n\n```\nfoo()\nbar()\n```\n\nEnd of doc.",
		},
		{
			name:     "Escape shortcodes in fenced code block",
			content:  "Some text before.\n\n```markdown\nThis is a {{< shortcode >}} example.\nAnd closing: {{< /shortcode >}}\n```\n\nSome text after.",
			expected: "Some text before.\n\n```markdown\nThis is a {{</* shortcode */>}} example.\nAnd closing: {{</* /shortcode */>}}\n```\n\nSome text after.",
		},
		{
			name:     "No shortcodes in code block",
			content:  "Some text.\n\n```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```",
			expected: "Some text.\n\n```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```",
		},
		{
			name:     "Shortcodes outside code block not escaped",
			content:  "Here's a {{< shortcode >}} in text.\n\n```markdown\nAnd one {{< inside >}} code.\n```\n\nAnother {{< outside >}} here.",
			expected: "Here's a {{< shortcode >}} in text.\n\n```markdown\nAnd one {{</* inside */>}} code.\n```\n\nAnother {{< outside >}} here.",
		},
		{
			name:     "Multiple code blocks",
			content:  "```\n{{< first >}}\n```\n\nText\n\n```\n{{< second >}}\n```",
			expected: "```\n{{</* first */>}}\n```\n\nText\n\n```\n{{</* second */>}}\n```",
		},
		{
			name:     "Tilde code blocks",
			content:  "~~~markdown\n{{< shortcode >}}\n~~~",
			expected: "~~~markdown\n{{</* shortcode */>}}\n~~~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{Content: tt.content}
			_, err := escapeShortcodesInCodeBlocks(doc)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, doc.Content)
		})
	}
}

func TestEscapeShortcodesInCodeBlocks_NoSpuriousChanges(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "Empty content",
			content: "",
		},
		{
			name:    "Plain text without shortcodes or code blocks",
			content: "This is just regular markdown text.\n\nWith multiple paragraphs.\n\nAnd no code blocks or shortcodes.",
		},
		{
			name:    "Content with shortcodes but no code blocks",
			content: "Here's a {{< ref \"page.md\" >}} link.\n\nAnd a {{< figure src=\"image.png\" >}} figure.\n\nNo code blocks here.",
		},
		{
			name:    "Code block without shortcodes",
			content: "```python\ndef hello():\n    print('world')\n```\n\nNo shortcodes in the code.",
		},
		{
			name:    "Already escaped shortcodes",
			content: "```markdown\n{{</* ref \"page\" */>}}\n```\n\nAlready escaped.",
		},
		{
			name:    "Content with special characters but no shortcodes",
			content: "```bash\necho \"<html>\" | grep '<'\ntest=\">}}\"\n```",
		},
		{
			name:    "Inline code with angle brackets",
			content: "Use `{{< shortcode >}}` in your templates.\n\nInline code should not be escaped.",
		},
		{
			name:    "Headers and lists",
			content: "# Title\n\n## Subtitle\n\n- Item 1\n- Item 2\n  - Nested\n\n1. First\n2. Second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{Content: tt.content}
			original := tt.content

			_, err := escapeShortcodesInCodeBlocks(doc)
			assert.NoError(t, err)
			assert.Equal(t, original, doc.Content, "Transform should not modify content that doesn't need escaping")
		})
	}
}

func TestEscapeShortcodesInCodeBlocks_Idempotent(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "Content with shortcodes in code blocks",
			content: "```markdown\n{{< shortcode >}}\n```",
		},
		{
			name:    "Multiple code blocks with shortcodes",
			content: "```\n{{< first >}}\n```\n\n```\n{{< second >}}\n```",
		},
		{
			name:    "Mixed content",
			content: "Text {{< outside >}}\n\n```\n{{< inside >}}\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{Content: tt.content}

			// First transformation
			_, err := escapeShortcodesInCodeBlocks(doc)
			assert.NoError(t, err)
			firstResult := doc.Content

			// Second transformation on already-transformed content
			_, err = escapeShortcodesInCodeBlocks(doc)
			assert.NoError(t, err)
			secondResult := doc.Content

			assert.Equal(t, firstResult, secondResult, "Transform should be idempotent")
		})
	}
}
