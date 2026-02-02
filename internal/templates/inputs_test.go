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

func TestParseInputValue_StringEnum_Valid(t *testing.T) {
	field := SchemaField{
		Key:     "Category",
		Type:    FieldTypeStringEnum,
		Options: []string{"a", "b", "c"},
	}
	value, hasValue, err := parseInputValue(field, "a")
	require.NoError(t, err)
	require.True(t, hasValue)
	require.Equal(t, "a", value)
}

func TestParseInputValue_StringEnum_Invalid(t *testing.T) {
	field := SchemaField{
		Key:     "Category",
		Type:    FieldTypeStringEnum,
		Options: []string{"a", "b", "c"},
	}
	_, _, err := parseInputValue(field, "d")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid value")
}

func TestParseInputValue_StringEnum_EmptyOptions(t *testing.T) {
	field := SchemaField{
		Key:     "Category",
		Type:    FieldTypeStringEnum,
		Options: []string{},
	}
	value, hasValue, err := parseInputValue(field, "any")
	require.NoError(t, err)
	require.True(t, hasValue)
	require.Equal(t, "any", value)
}

func TestParseInputValue_Bool_True(t *testing.T) {
	field := SchemaField{Key: "Published", Type: FieldTypeBool}
	value, hasValue, err := parseInputValue(field, "true")
	require.NoError(t, err)
	require.True(t, hasValue)
	require.Equal(t, true, value)
}

func TestParseInputValue_Bool_False(t *testing.T) {
	field := SchemaField{Key: "Published", Type: FieldTypeBool}
	value, hasValue, err := parseInputValue(field, "false")
	require.NoError(t, err)
	require.True(t, hasValue)
	require.Equal(t, false, value)
}

func TestParseInputValue_Bool_Yes(t *testing.T) {
	field := SchemaField{Key: "Published", Type: FieldTypeBool}
	// strconv.ParseBool only accepts "true", "false", "1", "0", "t", "f", "TRUE", "FALSE", "True", "False", "T", "F"
	// "yes"/"no" are not valid, so this should error
	_, _, err := parseInputValue(field, "yes")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid boolean")
}

func TestParseInputValue_Bool_No(t *testing.T) {
	field := SchemaField{Key: "Published", Type: FieldTypeBool}
	// strconv.ParseBool only accepts "true", "false", "1", "0", "t", "f", "TRUE", "FALSE", "True", "False", "T", "F"
	// "yes"/"no" are not valid, so this should error
	_, _, err := parseInputValue(field, "no")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid boolean")
}

func TestParseInputValue_Bool_T(t *testing.T) {
	field := SchemaField{Key: "Published", Type: FieldTypeBool}
	value, hasValue, err := parseInputValue(field, "t")
	require.NoError(t, err)
	require.True(t, hasValue)
	require.Equal(t, true, value)
}

func TestParseInputValue_Bool_F(t *testing.T) {
	field := SchemaField{Key: "Published", Type: FieldTypeBool}
	value, hasValue, err := parseInputValue(field, "f")
	require.NoError(t, err)
	require.True(t, hasValue)
	require.Equal(t, false, value)
}

func TestParseInputValue_Bool_Invalid(t *testing.T) {
	field := SchemaField{Key: "Published", Type: FieldTypeBool}
	_, _, err := parseInputValue(field, "maybe")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid boolean")
}

func TestParseInputValue_StringList_Empty(t *testing.T) {
	field := SchemaField{Key: "Tags", Type: FieldTypeStringList}
	_, hasValue, err := parseInputValue(field, "")
	require.NoError(t, err)
	require.False(t, hasValue)
}

func TestParseInputValue_StringList_Whitespace(t *testing.T) {
	field := SchemaField{Key: "Tags", Type: FieldTypeStringList}
	_, hasValue, err := parseInputValue(field, "   ")
	require.NoError(t, err)
	require.False(t, hasValue)
}

func TestParseInputValue_UnsupportedType(t *testing.T) {
	field := SchemaField{Key: "Unknown", Type: FieldType("unknown")}
	_, _, err := parseInputValue(field, "value")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported field type")
}
