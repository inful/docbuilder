package templates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSequenceDefinition(t *testing.T) {
	raw := `{"name":"adr","dir":"adr","glob":"adr-*.md","regex":"^adr-(\\d{3})-","start":1}`

	def, err := ParseSequenceDefinition(raw)
	require.NoError(t, err)
	require.NotNil(t, def)
	require.Equal(t, "adr", def.Name)
	require.Equal(t, "adr", def.Dir)
	require.Equal(t, "adr-*.md", def.Glob)
	require.Equal(t, "^adr-(\\d{3})-", def.Regex)
	require.Equal(t, 1, def.Start)
}

func TestParseSequenceDefinition_MissingRequired(t *testing.T) {
	raw := `{"dir":"adr","glob":"adr-*.md","regex":"^adr-(\\d{3})-"}`

	_, err := ParseSequenceDefinition(raw)
	require.Error(t, err)
}
