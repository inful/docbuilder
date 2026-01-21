package markdown

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractLinks_InlineLink(t *testing.T) {
	links, err := ExtractLinks([]byte("See [API](api.md) for details."), Options{})
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, LinkKindInline, links[0].Kind)
	require.Equal(t, "api.md", links[0].Destination)
}

func TestExtractLinks_ImageLink(t *testing.T) {
	links, err := ExtractLinks([]byte("![Diagram](diagram.png)"), Options{})
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, LinkKindImage, links[0].Kind)
	require.Equal(t, "diagram.png", links[0].Destination)
}

func TestExtractLinks_AutoLink(t *testing.T) {
	links, err := ExtractLinks([]byte("<https://example.com/path>"), Options{})
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, LinkKindAuto, links[0].Kind)
	require.Equal(t, "https://example.com/path", links[0].Destination)
}

func TestExtractLinks_ReferenceLinkUsageAndDefinition(t *testing.T) {
	src := []byte("See [API][ref].\n\n[ref]: api.md\n")
	links, err := ExtractLinks(src, Options{})
	require.NoError(t, err)

	// Expect one resolved link (Goldmark represents reference links as Link nodes with a Destination)
	// and one reference definition.
	require.Len(t, links, 2)
	require.Equal(t, LinkKindInline, links[0].Kind)
	require.Equal(t, "api.md", links[0].Destination)
	require.Equal(t, LinkKindReferenceDefinition, links[1].Kind)
	require.Equal(t, "api.md", links[1].Destination)
}

func TestExtractLinks_SkipsInlineCodeAndCodeBlocks(t *testing.T) {
	src := []byte("" +
		"Inline code: `[Link](./ignored-inline.md)`\n" +
		"\n" +
		"```\n" +
		"[Link](./ignored-fence.md)\n" +
		"```\n" +
		"\n" +
		"Real: [OK](./real.md)\n")

	links, err := ExtractLinks(src, Options{})
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, "./real.md", links[0].Destination)
}
