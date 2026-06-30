package templates

import (
	"encoding/json"
	"strings"

	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// ParseTemplateSchema parses a JSON string into a TemplateSchema structure.
//
// The JSON is extracted from the "docbuilder:template.schema" meta tag and defines
// all input fields required to instantiate a template.
//
// Parameters:
//   - raw: JSON string containing the schema definition
//
// Returns:
//   - A parsed TemplateSchema with all fields
//   - An empty TemplateSchema (no error) if raw is empty
//   - An error if JSON is invalid
//
// Example:
//
//	json := `{"fields":[{"key":"Title","type":"string","required":true}]}`
//	schema, err := ParseTemplateSchema(json)
func ParseTemplateSchema(raw string) (TemplateSchema, error) {
	if strings.TrimSpace(raw) == "" {
		return TemplateSchema{}, nil
	}

	var schema TemplateSchema
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		return TemplateSchema{}, derrors.WrapError(err, derrors.CategoryValidation, "parse template schema").Build()
	}
	return schema, nil
}

// ParseTemplateDefaults parses a JSON string into a map of default values.
//
// The JSON is extracted from the "docbuilder:template.defaults" meta tag and provides
// default values for template fields that can be overridden by user input.
//
// Parameters:
//   - raw: JSON object string with key-value pairs
//
// Returns:
//   - A map of field keys to default values (strings, numbers, bools, arrays)
//   - An empty map (no error) if raw is empty
//   - An error if JSON is invalid
//
// Example:
//
//	json := `{"categories":["architecture-decisions"],"tags":["adr"]}`
//	defaults, err := ParseTemplateDefaults(json)
func ParseTemplateDefaults(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var defaults map[string]any
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		return nil, derrors.WrapError(err, derrors.CategoryValidation, "parse template defaults").Build()
	}
	return defaults, nil
}
