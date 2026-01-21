package docmodel

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"github.com/stretchr/testify/require"
)

func TestParse_NoFrontmatter_RoundTrip(t *testing.T) {
	content := []byte("# Hello\n\nBody\n")

	doc, err := Parse(content, Options{})
	require.NoError(t, err)
	require.False(t, doc.HadFrontmatter())
	require.Nil(t, doc.FrontmatterRaw())
	require.Equal(t, content, doc.Body())
	require.Equal(t, content, doc.Bytes())
}

func TestParse_EmptyFrontmatter_RoundTrip(t *testing.T) {
	content := []byte("---\n---\n# Hi\n")

	doc, err := Parse(content, Options{})
	require.NoError(t, err)
	require.True(t, doc.HadFrontmatter())
	require.Equal(t, []byte{}, doc.FrontmatterRaw())
	require.Equal(t, []byte("# Hi\n"), doc.Body())
	require.Equal(t, content, doc.Bytes())
}

func TestParse_MissingClosingDelimiter_ReturnsFrontmatterError(t *testing.T) {
	content := []byte("---\nkey: value\n# body\n")

	_, err := Parse(content, Options{})
	require.Error(t, err)
	require.ErrorIs(t, err, frontmatter.ErrMissingClosingDelimiter)
}

func TestParse_CapturesStyle(t *testing.T) {
	content := []byte("---\r\nkey: value\r\n---\r\n# body\r\n")

	doc, err := Parse(content, Options{})
	require.NoError(t, err)
	require.True(t, doc.HadFrontmatter())
	style := doc.Style()
	require.Equal(t, "\r\n", style.Newline)
	require.True(t, style.HasTrailingNewline)
	require.Equal(t, content, doc.Bytes())
}

func TestParseFile_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "doc.md")
	content := []byte("---\nkey: value\n---\n# Title\n")
	require.NoError(t, os.WriteFile(path, content, 0o600))

	doc, err := ParseFile(path, Options{})
	require.NoError(t, err)
	require.Equal(t, content, doc.Bytes())
}

func TestParsedDoc_DoesNotExposeMutableBytes_NoFrontmatter(t *testing.T) {
	content := []byte("# Hello\n\nBody\n")

	doc, err := Parse(content, Options{})
	require.NoError(t, err)

	buf := doc.Bytes()
	require.Equal(t, byte('#'), buf[0])
	buf[0] = 'X'

	// Re-reading bytes should not reflect mutation.
	buf2 := doc.Bytes()
	require.Equal(t, byte('#'), buf2[0])

	// Body should also remain unchanged.
	body := doc.Body()
	require.Equal(t, byte('#'), body[0])
}
