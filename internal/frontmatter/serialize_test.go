package frontmatter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSerializeYAML_EmptyMap_ReturnsEmpty(t *testing.T) {
	out, err := SerializeYAML(map[string]any{}, Style{Newline: "\n"})
	require.NoError(t, err)
	require.Equal(t, "", string(out))
}

func TestSerializeYAML_DeterministicOrderAndTrailingNewline(t *testing.T) {
	fields := map[string]any{
		"b": "two",
		"a": "one",
		"c": 3,
	}

	out1, err := SerializeYAML(fields, Style{Newline: "\n"})
	require.NoError(t, err)
	out2, err := SerializeYAML(fields, Style{Newline: "\n"})
	require.NoError(t, err)
	// Must be stable across runs.
	require.Equal(t, string(out1), string(out2))

	// Deterministic key ordering and trailing newline.
	require.Equal(t, "a: one\nb: two\nc: 3\n", string(out1))
}

func TestSerializeYAML_NewlineStyle_CRLF(t *testing.T) {
	fields := map[string]any{"a": "one"}
	out, err := SerializeYAML(fields, Style{Newline: "\r\n"})
	require.NoError(t, err)
	require.Equal(t, "a: one\r\n", string(out))
}

func TestSerializeYAML_NestedMap_SortsKeysRecursively(t *testing.T) {
	fields := map[string]any{
		"outer": map[string]any{
			"b": 2,
			"a": 1,
		},
	}

	out, err := SerializeYAML(fields, Style{Newline: "\n"})
	require.NoError(t, err)
	require.Equal(t, "outer:\n  a: 1\n  b: 2\n", string(out))
}
