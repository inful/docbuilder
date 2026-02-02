package templates

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubPrompter struct {
	responses map[string]string
	calls     []string
}

func (s *stubPrompter) Prompt(field SchemaField) (string, error) {
	s.calls = append(s.calls, field.Key)
	if value, ok := s.responses[field.Key]; ok {
		return value, nil
	}
	return "", errors.New("missing response")
}

func TestResolveTemplateInputs_UsesDefaultsAndOverrides(t *testing.T) {
	schema := TemplateSchema{
		Fields: []SchemaField{
			{Key: "Title", Type: FieldTypeString, Required: true},
			{Key: "Tags", Type: FieldTypeStringList, Required: false},
			{Key: "Published", Type: FieldTypeBool, Required: false},
		},
	}

	defaults := map[string]any{
		"Title":     "Default Title",
		"Tags":      []string{"docs", "adr"},
		"Published": true,
	}
	overrides := map[string]string{
		"Title": "Override Title",
	}

	got, err := ResolveTemplateInputs(schema, defaults, overrides, true, &stubPrompter{})
	require.NoError(t, err)
	require.Equal(t, "Override Title", got["Title"])
	require.Equal(t, []string{"docs", "adr"}, got["Tags"])
	require.Equal(t, true, got["Published"])
}

func TestResolveTemplateInputs_PromptsRequiredFields(t *testing.T) {
	schema := TemplateSchema{
		Fields: []SchemaField{
			{Key: "Title", Type: FieldTypeString, Required: true},
			{Key: "Tags", Type: FieldTypeStringList, Required: false},
			{Key: "Approved", Type: FieldTypeBool, Required: true},
		},
	}

	prompter := &stubPrompter{
		responses: map[string]string{
			"Title":    "My Title",
			"Tags":     "docs, adr",
			"Approved": "true",
		},
	}

	got, err := ResolveTemplateInputs(schema, nil, nil, false, prompter)
	require.NoError(t, err)
	require.Equal(t, "My Title", got["Title"])
	require.Equal(t, []string{"docs", "adr"}, got["Tags"])
	require.Equal(t, true, got["Approved"])
	require.ElementsMatch(t, []string{"Title", "Tags", "Approved"}, prompter.calls)
}

func TestResolveTemplateInputs_NonInteractiveMissingRequired(t *testing.T) {
	schema := TemplateSchema{
		Fields: []SchemaField{
			{Key: "Title", Type: FieldTypeString, Required: true},
		},
	}

	_, err := ResolveTemplateInputs(schema, nil, nil, true, nil)
	require.Error(t, err)
}
