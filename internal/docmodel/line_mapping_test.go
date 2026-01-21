package docmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsedDoc_LineOffset_NoFrontmatter(t *testing.T) {
	doc, err := Parse([]byte("# Title\n"), Options{})
	require.NoError(t, err)
	assert.Equal(t, 0, doc.LineOffset())
}

func TestParsedDoc_LineOffset_WithFrontmatter(t *testing.T) {
	content := "---\n" +
		"title: x\n" +
		"---\n" +
		"# Body\n"

	doc, err := Parse([]byte(content), Options{})
	require.NoError(t, err)

	// Body starts on line 4 in the original file.
	assert.Equal(t, 3, doc.LineOffset())
	assert.Equal(t, 4, doc.LineOffset()+1)
}

func TestParsedDoc_FindNextLineContaining_SkipsCodeBlocksAndInlineCode(t *testing.T) {
	body := "" +
		"```sh\n" +
		"echo ./missing.md\n" +
		"```\n" +
		"Use `./missing.md` as an example.\n" +
		"Real link: [Missing](./missing.md)\n"

	doc, err := Parse([]byte(body), Options{})
	require.NoError(t, err)

	line := doc.FindNextLineContaining("./missing.md", 1)
	assert.Equal(t, 5, line)
}

func TestParsedDoc_FindNextLineContaining_RespectsStartLine(t *testing.T) {
	body := "" +
		"First: [Missing](./missing.md)\n" +
		"Second: [Missing](./missing.md)\n"

	doc, err := Parse([]byte(body), Options{})
	require.NoError(t, err)

	first := doc.FindNextLineContaining("./missing.md", 1)
	second := doc.FindNextLineContaining("./missing.md", first+1)

	assert.Equal(t, 1, first)
	assert.Equal(t, 2, second)
}
