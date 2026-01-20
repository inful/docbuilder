package frontmatter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplit_NoFrontmatter_ReturnsBodyOnly(t *testing.T) {
	input := []byte("# Title\n\nHello\n")

	fm, body, had, _, err := Split(input)
	require.NoError(t, err)
	require.False(t, had)
	require.Empty(t, fm)
	require.Equal(t, input, body)
}

func TestSplit_YAMLFrontmatter_SplitsFrontmatterAndBody(t *testing.T) {
	input := []byte("---\nkey: value\n---\n# Title\n")

	fm, body, had, _, err := Split(input)
	require.NoError(t, err)
	require.True(t, had)
	require.Equal(t, []byte("key: value\n"), fm)
	require.Equal(t, []byte("# Title\n"), body)
}

func TestSplit_MissingClosingDelimiter_ReturnsError(t *testing.T) {
	input := []byte("---\nkey: value\n# Title\n")

	fm, body, had, style, err := Split(input)
	_ = fm
	_ = body
	_ = style
	require.Error(t, err)
	require.False(t, had)
	require.True(t, errors.Is(err, ErrMissingClosingDelimiter))
}

func TestSplit_CRLF_SplitsFrontmatterAndBody(t *testing.T) {
	input := []byte("---\r\nkey: value\r\n---\r\n# Title\r\n")

	fm, body, had, _, err := Split(input)
	require.NoError(t, err)
	require.True(t, had)
	require.Equal(t, []byte("key: value\r\n"), fm)
	require.Equal(t, []byte("# Title\r\n"), body)
}

func TestSplit_EmptyFrontmatterBlock_SplitsAsHadWithEmptyFrontmatter(t *testing.T) {
	input := []byte("---\n---\n# Title\n")

	fm, body, had, _, err := Split(input)
	require.NoError(t, err)
	require.True(t, had)
	require.Empty(t, fm)
	require.Equal(t, []byte("# Title\n"), body)
}

func TestJoin_RoundTrip_ReconstructsOriginalBytes(t *testing.T) {
	cases := [][]byte{
		[]byte("# Title\n\nHello\n"),
		[]byte("---\nkey: value\n---\n# Title\n"),
		[]byte("---\n---\n# Title\n"),
		[]byte("---\r\nkey: value\r\n---\r\n# Title\r\n"),
	}

	for _, input := range cases {
		fm, body, had, style, err := Split(input)
		require.NoError(t, err)

		out := Join(fm, body, had, style)
		require.Equal(t, input, out)
	}
}

func TestParseYAML_ValidYAML_ReturnsMap(t *testing.T) {
	fm := []byte("uid: abc\ntags:\n  - one\n")

	fields, err := ParseYAML(fm)
	require.NoError(t, err)
	require.Equal(t, "abc", fields["uid"])
	require.Equal(t, []any{"one"}, fields["tags"])
}

func TestParseYAML_Empty_ReturnsEmptyMap(t *testing.T) {
	fields, err := ParseYAML(nil)
	require.NoError(t, err)
	require.Empty(t, fields)
}

func TestParseYAML_InvalidYAML_ReturnsError(t *testing.T) {
	_, err := ParseYAML([]byte(": not yaml"))
	require.Error(t, err)
}
