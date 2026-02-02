package templates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTemplateSchema(t *testing.T) {
	raw := `{"fields":[{"key":"Title","type":"string","required":true},{"key":"Kind","type":"string_enum","options":["adr","tech"]}]}`

	schema, err := ParseTemplateSchema(raw)
	require.NoError(t, err)
	require.Len(t, schema.Fields, 2)
	require.Equal(t, "Title", schema.Fields[0].Key)
	require.Equal(t, FieldTypeString, schema.Fields[0].Type)
	require.True(t, schema.Fields[0].Required)
	require.Equal(t, []string{"adr", "tech"}, schema.Fields[1].Options)
}

func TestParseTemplateDefaults(t *testing.T) {
	raw := `{"Title":"My Title","Tags":["a","b"],"Published":true}`

	defaults, err := ParseTemplateDefaults(raw)
	require.NoError(t, err)
	require.Equal(t, "My Title", defaults["Title"])
	require.Equal(t, []any{"a", "b"}, defaults["Tags"])
	require.Equal(t, true, defaults["Published"])
}

func TestParseTemplateSchema_Empty(t *testing.T) {
	schema, err := ParseTemplateSchema("")
	require.NoError(t, err)
	require.Len(t, schema.Fields, 0)
}

func TestParseTemplateSchema_InvalidJSON(t *testing.T) {
	_, err := ParseTemplateSchema(`{"fields":[{"key":"Title"`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse template schema")
}

func TestParseTemplateDefaults_Empty(t *testing.T) {
	defaults, err := ParseTemplateDefaults("")
	require.NoError(t, err)
	require.Len(t, defaults, 0)
}

func TestParseTemplateDefaults_InvalidJSON(t *testing.T) {
	_, err := ParseTemplateDefaults(`{"Title":"My Title"`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse template defaults")
}
