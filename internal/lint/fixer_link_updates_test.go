package lint

import (
	"bytes"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/markdown"
	"github.com/stretchr/testify/require"
)

func TestFindLineByteRange_CRLF(t *testing.T) {
	content := []byte("first\r\nsecond\r\nthird\r\n")

	start, end, ok := findLineByteRange(content, 2)
	require.True(t, ok)
	require.Equal(t, len("first\r\n"), start)
	// end should be the index of the '\n' byte for the second line
	require.Equal(t, len("first\r\nsecond\r"), end)
	require.Less(t, end, len(content))
	require.Equal(t, byte('\n'), content[end])
	require.Equal(t, byte('\r'), content[end-1])
	require.Equal(t, "second\r", string(content[start:end]))
}

func TestFindLineByteRange_LF(t *testing.T) {
	content := []byte("first\nsecond\nthird\n")

	start, end, ok := findLineByteRange(content, 2)
	require.True(t, ok)
	require.Equal(t, len("first\n"), start)
	require.Equal(t, len("first\nsecond"), end)
	require.Less(t, end, len(content))
	require.Equal(t, byte('\n'), content[end])
	require.Equal(t, "second", string(content[start:end]))
}

func TestByteRangeEdit_UsingCRLFLineRanges(t *testing.T) {
	content := []byte("intro\r\nSee [Doc](./old.md) here.\r\noutro\r\n")

	lineStart, lineEnd, ok := findLineByteRange(content, 2)
	require.True(t, ok)

	line := content[lineStart:lineEnd]
	old := []byte("./old.md")
	idx := bytes.Index(line, old)
	require.NotEqual(t, -1, idx)

	out, err := markdown.ApplyEdits(content, []markdown.Edit{{
		Start:       lineStart + idx,
		End:         lineStart + idx + len(old),
		Replacement: []byte("./new.md"),
	}})
	require.NoError(t, err)
	require.Equal(t, "intro\r\nSee [Doc](./new.md) here.\r\noutro\r\n", string(out))
}
