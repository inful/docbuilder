package frontmatter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSerializeYAML_EmptyMap_ReturnsEmpty(t *testing.T) {
	out, err := SerializeYAML(map[string]any{}, Style{Newline: "\n"})
	require.NoError(t, err)
	require.Equal(t, "", string(out))
}

func TestSerializeYAML_DeterministicOrderAndTrailingNewline(t *testing.T) {
	fields := map[string]any{
		"b": "two",
		"a": testValueOne,
		"c": 3,
	}

	out1, err := SerializeYAML(fields, Style{Newline: "\n"})
	require.NoError(t, err)
	out2, err := SerializeYAML(fields, Style{Newline: "\n"})
	require.NoError(t, err)
	// Must be stable across runs.
	require.Equal(t, string(out1), string(out2))

	// Deterministic key ordering and trailing newline.
	require.Equal(t, "a: "+testValueOne+"\nb: two\nc: 3\n", string(out1))
}

func TestSerializeYAML_NewlineStyle_CRLF(t *testing.T) {
	fields := map[string]any{"a": testValueOne}
	out, err := SerializeYAML(fields, Style{Newline: "\r\n"})
	require.NoError(t, err)
	require.Equal(t, "a: "+testValueOne+"\r\n", string(out))
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

func TestNodeFromAny(t *testing.T) {

	cases := []struct {

		name string

		in   any

		kind yaml.Kind

		tag  string

		val  string

	}{

		{name: "nil", in: nil, kind: yaml.ScalarNode, tag: "!!null", val: "null"},

		{name: "string", in: "hello", kind: yaml.ScalarNode, tag: "!!str", val: "hello"},

		{name: "empty string", in: "", kind: yaml.ScalarNode, tag: "!!str", val: ""},

		{name: "bool true", in: true, kind: yaml.ScalarNode, tag: "!!bool", val: "true"},

		{name: "bool false", in: false, kind: yaml.ScalarNode, tag: "!!bool", val: "false"},

		{name: "int", in: 42, kind: yaml.ScalarNode, tag: "!!int", val: "42"},

		{name: "int64", in: int64(123456789012), kind: yaml.ScalarNode, tag: "!!int", val: "123456789012"},

		{name: "float64", in: 3.14, kind: yaml.ScalarNode, tag: "!!float", val: "3.14"},

	}



	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {

			got, err := nodeFromAny(tc.in)

			require.NoError(t, err)

			require.NotNil(t, got)

			require.Equal(t, tc.kind, got.Kind, "kind")

			require.Equal(t, tc.tag, got.Tag, "tag")

			require.Equal(t, tc.val, got.Value, "value")

		})

	}

}



func TestNodeFromAny_StringSequence(t *testing.T) {

	got, err := nodeFromAny([]string{"a", "b", "c"})

	require.NoError(t, err)

	require.Equal(t, yaml.SequenceNode, got.Kind)

	require.Len(t, got.Content, 3)

	for i, want := range []string{"a", "b", "c"} {

		require.Equal(t, yaml.ScalarNode, got.Content[i].Kind)

		require.Equal(t, "!!str", got.Content[i].Tag)

		require.Equal(t, want, got.Content[i].Value)

	}

}



func TestNodeFromAny_AnySequenceIsRecursive(t *testing.T) {

	got, err := nodeFromAny([]any{"x", 1, true})

	require.NoError(t, err)

	require.Equal(t, yaml.SequenceNode, got.Kind)

	require.Len(t, got.Content, 3)

	require.Equal(t, "x", got.Content[0].Value)

	require.Equal(t, "1", got.Content[1].Value)

	require.Equal(t, "true", got.Content[2].Value)

}



func TestNodeFromAny_StringMapRecurses(t *testing.T) {

	got, err := nodeFromAny(map[string]any{"name": "docbuilder", "count": 3})

	require.NoError(t, err)

	require.Equal(t, yaml.MappingNode, got.Kind)

	require.Len(t, got.Content, 4) // 2 key-value pairs (each as 2 nodes)

}



func TestNodeFromAny_AnyMapKeysAreStringified(t *testing.T) {

	got, err := nodeFromAny(map[any]any{1: "one", "two": 2})

	require.NoError(t, err)

	require.Equal(t, yaml.MappingNode, got.Kind)

	require.Len(t, got.Content, 4) // 2 entries

	// Collect the stringified keys.

	keys := make(map[string]bool)

	for i := 0; i < len(got.Content); i += 2 {

		keys[got.Content[i].Value] = true

	}

	require.True(t, keys["1"], "int key 1 should be stringified")

	require.True(t, keys["two"], "string key should pass through")

}



func TestNodeFromAny_FallsBackToYAMLEncoderForUnknownTypes(t *testing.T) {

	type custom struct{ Value int }

	got, err := nodeFromAny(custom{Value: 7})

	require.NoError(t, err)

	require.NotNil(t, got)

	require.Equal(t, yaml.MappingNode, got.Kind)

	require.NotEmpty(t, got.Content)

}


