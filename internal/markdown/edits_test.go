package markdown

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyEdits_SingleReplacement(t *testing.T) {
	src := []byte("See [API](./api-guide.md) for details.\n")
	old := []byte("./api-guide.md")
	idx := bytes.Index(src, old)
	require.NotEqual(t, -1, idx)

	out, err := ApplyEdits(src, []Edit{{Start: idx, End: idx + len(old), Replacement: []byte("./api_guide.md")}})
	require.NoError(t, err)
	require.Equal(t, "See [API](./api_guide.md) for details.\n", string(out))
}

func TestApplyEdits_MultipleReplacements(t *testing.T) {
	src := []byte("A: ./old.md\nB: ./old.md#frag\n")

	idx1 := bytes.Index(src, []byte("./old.md"))
	require.NotEqual(t, -1, idx1)

	idx2 := bytes.LastIndex(src, []byte("./old.md#frag"))
	require.NotEqual(t, -1, idx2)

	out, err := ApplyEdits(src, []Edit{
		{Start: idx1, End: idx1 + len("./old.md"), Replacement: []byte("./new.md")},
		{Start: idx2, End: idx2 + len("./old.md#frag"), Replacement: []byte("./new.md#frag")},
	})
	require.NoError(t, err)
	require.Equal(t, "A: ./new.md\nB: ./new.md#frag\n", string(out))
}

func TestApplyEdits_CRLFInputPreserved(t *testing.T) {
	src := []byte("A: ./old.md\r\nB: ./old.md\r\n")

	idx := bytes.Index(src, []byte("./old.md"))
	require.NotEqual(t, -1, idx)

	out, err := ApplyEdits(src, []Edit{{
		Start:       idx,
		End:         idx + len("./old.md"),
		Replacement: []byte("./new.md"),
	}})
	require.NoError(t, err)
	require.Equal(t, "A: ./new.md\r\nB: ./old.md\r\n", string(out))
}

func TestApplyEdits_ReferenceDefinitionReplacement(t *testing.T) {
	src := []byte("Reference: [api][1]\n\n[1]: ./api-guide.md \"Title\"\n")
	old := []byte("./api-guide.md")
	idx := bytes.Index(src, old)
	require.NotEqual(t, -1, idx)

	out, err := ApplyEdits(src, []Edit{{Start: idx, End: idx + len(old), Replacement: []byte("./api_guide.md")}})
	require.NoError(t, err)
	require.Contains(t, string(out), "[1]: ./api_guide.md \"Title\"")
}

func TestApplyEdits_RejectsOverlappingEdits(t *testing.T) {
	src := []byte("abcdef")
	_, err := ApplyEdits(src, []Edit{
		{Start: 1, End: 4, Replacement: []byte("X")},
		{Start: 3, End: 5, Replacement: []byte("Y")},
	})
	require.Error(t, err)
}
