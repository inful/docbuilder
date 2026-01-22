package frontmatterops

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

func TestRead_NoFrontmatter_ReturnsEmptyFieldsAndBody(t *testing.T) {
	input := []byte("# Title\n\nHello\n")

	fields, body, had, style, err := Read(input)
	require.NoError(t, err)
	require.False(t, had)
	require.NotNil(t, fields)
	require.Empty(t, fields)
	require.Equal(t, input, body)
	require.Equal(t, "\n", style.Newline)
}

func TestRead_EmptyFrontmatterBlock_ReturnsHadWithEmptyFields(t *testing.T) {
	input := []byte("---\n---\n# Title\n")

	fields, body, had, style, err := Read(input)
	require.NoError(t, err)
	require.True(t, had)
	require.NotNil(t, fields)
	require.Empty(t, fields)
	require.Equal(t, []byte("# Title\n"), body)
	require.Equal(t, "\n", style.Newline)
}

func TestRead_ValidYAMLFrontmatter_ReturnsFieldsAndBody(t *testing.T) {
	input := []byte("---\nuid: abc\ntags:\n  - one\n---\n# Title\n")

	fields, body, had, _, err := Read(input)
	require.NoError(t, err)
	require.True(t, had)
	require.Equal(t, "abc", fields["uid"])
	require.Equal(t, []any{"one"}, fields["tags"])
	require.Equal(t, []byte("# Title\n"), body)
}

func TestRead_InvalidYAML_ReturnsError(t *testing.T) {
	input := []byte("---\n: not yaml\n---\n# Title\n")

	_, _, _, _, err := Read(input)
	require.Error(t, err)
}

func TestRead_MissingClosingDelimiter_ReturnsError(t *testing.T) {
	input := []byte("---\nkey: value\n# Title\n")

	_, _, had, _, err := Read(input)
	require.Error(t, err)
	require.False(t, had)
	require.True(t, errors.Is(err, frontmatter.ErrMissingClosingDelimiter))
}

func TestWrite_HadFalse_ReturnsBodyOnly(t *testing.T) {
	fields := map[string]any{"uid": "abc"}
	body := []byte("# Title\n")

	out, err := Write(fields, body, false, frontmatter.Style{Newline: "\n"})
	require.NoError(t, err)
	require.Equal(t, body, out)
}

func TestWrite_HadTrue_EmitsYAMLFrontmatterAndBody(t *testing.T) {
	fields := map[string]any{"b": "two", "a": "one"}
	body := []byte("# Title\n")

	out, err := Write(fields, body, true, frontmatter.Style{Newline: "\n"})
	require.NoError(t, err)
	require.Equal(t, []byte("---\na: one\nb: two\n---\n# Title\n"), out)
}

func TestWrite_HadTrue_EmptyFields_EmitsEmptyFrontmatterBlock(t *testing.T) {
	out, err := Write(map[string]any{}, []byte("# Title\n"), true, frontmatter.Style{Newline: "\n"})
	require.NoError(t, err)
	require.Equal(t, []byte("---\n---\n# Title\n"), out)
}

func TestWrite_CRLFStyle_UsesCRLFDelimitersAndNewlines(t *testing.T) {
	fields := map[string]any{"uid": "abc"}
	body := []byte("# Title\r\n")

	out, err := Write(fields, body, true, frontmatter.Style{Newline: "\r\n"})
	require.NoError(t, err)
	require.Equal(t, []byte("---\r\nuid: abc\r\n---\r\n# Title\r\n"), out)
}
