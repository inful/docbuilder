package docmodel

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/markdown"
)

func TestParsedDoc_Links_ParityWithMarkdownExtractLinks(t *testing.T) {
	src := "# Title\n\n" +
		"See [API](api.md) and ![Diagram](diagram.png).\n" +
		"<https://example.com/path>\n" +
		"[ref]: ref.md\n" +
		"```\n" +
		"[Ignored](ignored.md)\n" +
		"```\n" +
		"Inline code `[Ignored2](ignored2.md)` should be ignored.\n"

	doc, err := Parse([]byte(src), Options{})
	require.NoError(t, err)

	got, err := doc.Links()
	require.NoError(t, err)

	expected, err := markdown.ExtractLinks(doc.Body(), markdown.Options{})
	require.NoError(t, err)

	require.Equal(t, expected, got)
}

func TestParsedDoc_LinkRefs_ComputesFileLineNumbersWithFrontmatterOffset(t *testing.T) {
	src := "---\n" +
		"title: x\n" +
		"---\n" +
		"[A](a.md)\n" +
		"[B](b.md)\n"

	doc, err := Parse([]byte(src), Options{})
	require.NoError(t, err)

	refs, err := doc.LinkRefs()
	require.NoError(t, err)

	require.Len(t, refs, 2)
	require.Equal(t, 1, refs[0].BodyLine)
	require.Equal(t, doc.LineOffset()+1, refs[0].FileLine)
	require.Equal(t, 2, refs[1].BodyLine)
	require.Equal(t, doc.LineOffset()+2, refs[1].FileLine)
}

func TestParsedDoc_LinkRefs_StableForRepeatedDestinations(t *testing.T) {
	src := "First: [X](a.md)\nSecond: [X](a.md)\n"
	doc, err := Parse([]byte(src), Options{})
	require.NoError(t, err)

	refs, err := doc.LinkRefs()
	require.NoError(t, err)

	require.Len(t, refs, 2)
	require.Equal(t, 1, refs[0].BodyLine)
	require.Equal(t, 2, refs[1].BodyLine)
}
