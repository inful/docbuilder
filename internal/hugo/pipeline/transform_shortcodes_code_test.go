package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A line that holds an escaped shortcode and another one: the second one
// is escaped too. A shortcode that reaches Hugo takes the whole site
// build down: "failed to extract shortcode: template for shortcode not
// found".
func TestEscapeShortcodes_EveryShortcodeOfALineIsEscaped(t *testing.T) {
	doc := &Document{Content: "```markdown\n{{</* ref \"a\" */>}} and {{< myshortcode >}}\n```\n"}

	_, err := escapeShortcodesInCodeBlocks(doc)

	assert.NoError(t, err)
	assert.Equal(t, "```markdown\n{{</* ref \"a\" */>}} and {{</* myshortcode */>}}\n```\n", doc.Content)
}

// An indented code block is code as well: the shortcode in it is an
// example, and Hugo would try to run it.
func TestEscapeShortcodes_IndentedCodeBlocks(t *testing.T) {
	doc := &Document{Content: "Text:\n\n    {{< myshortcode >}}\n"}

	_, err := escapeShortcodesInCodeBlocks(doc)

	assert.NoError(t, err)
	assert.Equal(t, "Text:\n\n    {{</* myshortcode */>}}\n", doc.Content)
}

// The document keeps its bytes: the transform does not add a newline at
// the end, so the fingerprint of a document does not change with it.
func TestEscapeShortcodes_TheContentKeepsItsBytes(t *testing.T) {
	content := "Plain body.\n"

	doc := &Document{Content: content}

	_, err := escapeShortcodesInCodeBlocks(doc)

	assert.NoError(t, err)
	assert.Equal(t, content, doc.Content)
}

// The transform can run twice: the second run changes nothing.
func TestEscapeShortcodes_TheTransformIsIdempotent(t *testing.T) {
	content := "```markdown\n{{< shortcode >}}\n```\n\nText.\n"

	doc := &Document{Content: content}
	_, err := escapeShortcodesInCodeBlocks(doc)
	assert.NoError(t, err)
	first := doc.Content

	_, err = escapeShortcodesInCodeBlocks(doc)
	assert.NoError(t, err)

	assert.Equal(t, first, doc.Content)
}
