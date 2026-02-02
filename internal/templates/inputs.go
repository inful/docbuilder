package templates

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// FieldType defines the supported input field types for template schemas.
type FieldType string

const (
	// FieldTypeString is a free-form text input.
	FieldTypeString FieldType = "string"

	// FieldTypeStringEnum is a selection from predefined options (requires Options field).
	FieldTypeStringEnum FieldType = "string_enum"

	// FieldTypeStringList is a comma-separated list of strings.
	FieldTypeStringList FieldType = "string_list"

	// FieldTypeBool is a boolean value (true/false, yes/no, etc.).
	FieldTypeBool FieldType = "bool"
)

// SchemaField represents a single input field in a template schema.
type SchemaField struct {
	// Key is the field identifier used in templates (e.g., "Title", "Slug").
	Key string `json:"key"`

	// Type determines how the field is prompted and validated.
	Type FieldType `json:"type"`

	// Required indicates whether the field must be provided.
	Required bool `json:"required"`

	// Options is required for FieldTypeStringEnum and lists valid choices.
	Options []string `json:"options,omitempty"`
}

// TemplateSchema describes all input fields required to instantiate a template.
//
// The schema is parsed from the "docbuilder:template.schema" meta tag JSON.
type TemplateSchema struct {
	Fields []SchemaField `json:"fields"`
}

// Prompter is an interface for interactively collecting user input for template fields.
//
// Implementations typically prompt via stdin/stdout, but can also use GUI dialogs
// or other input mechanisms.
type Prompter interface {
	// Prompt requests input for a field and returns the user's response.
	// An empty string indicates the user skipped the field (if not required).
	Prompt(field SchemaField) (string, error)
}

// ResolveTemplateInputs resolves all template inputs by merging defaults, overrides, and prompts.
//
// The resolution order is:
//  1. Apply defaults from template metadata
//  2. Apply overrides (from --set flags, highest precedence)
//  3. If useDefaults is true, validate required fields and return
//  4. Otherwise, prompt for missing fields using the Prompter
//  5. Validate all required fields are present
//
// Parameters:
//   - schema: The template schema defining all fields
//   - defaults: Default values from template metadata (JSON parsed)
//   - overrides: User-provided overrides (e.g., from CLI flags)
//   - useDefaults: If true, skip prompting and use defaults only
//   - prompter: Interface for collecting user input (nil if non-interactive)
//
// Returns:
//   - A map of field keys to resolved values (strings, bools, or string slices)
//   - An error if required fields are missing or validation fails
//
// Example:
//
//	schema := TemplateSchema{Fields: []SchemaField{
//	    {Key: "Title", Type: FieldTypeString, Required: true},
//	    {Key: "Category", Type: FieldTypeStringEnum, Required: true, Options: []string{"a", "b"}},
//	}}
//	defaults := map[string]any{"Category": "a"}
//	overrides := map[string]string{"Title": "My Document"}
//	inputs, err := ResolveTemplateInputs(schema, defaults, overrides, false, myPrompter)
func ResolveTemplateInputs(schema TemplateSchema, defaults map[string]any, overrides map[string]string, useDefaults bool, prompter Prompter) (map[string]any, error) {
	result := make(map[string]any)
	fieldsByKey := make(map[string]SchemaField)
	for _, field := range schema.Fields {
		fieldsByKey[field.Key] = field
	}

	for key, value := range defaults {
		if value != nil {
			result[key] = value
		}
	}

	for key, value := range overrides {
		if field, ok := fieldsByKey[key]; ok {
			parsed, hasValue, err := parseInputValue(field, value)
			if err != nil {
				return nil, err
			}
			if hasValue {
				result[key] = parsed
			}
			continue
		}
		result[key] = value
	}

	if useDefaults {
		if err := validateRequiredFields(schema, result); err != nil {
			return nil, err
		}
		return result, nil
	}

	if prompter == nil {
		return nil, errors.New("prompter is required when defaults are not used")
	}

	for _, field := range schema.Fields {
		if _, ok := result[field.Key]; ok {
			continue
		}
		response, err := prompter.Prompt(field)
		if err != nil {
			return nil, err
		}
		parsed, hasValue, err := parseInputValue(field, response)
		if err != nil {
			return nil, err
		}
		if hasValue {
			result[field.Key] = parsed
		}
	}

	if err := validateRequiredFields(schema, result); err != nil {
		return nil, err
	}

	return result, nil
}

// validateRequiredFields ensures all required fields in the schema have values.
func validateRequiredFields(schema TemplateSchema, values map[string]any) error {
	for _, field := range schema.Fields {
		if !field.Required {
			continue
		}
		value, ok := values[field.Key]
		if !ok || value == nil {
			return fmt.Errorf("missing required field: %s", field.Key)
		}
	}
	return nil
}

// parseInputValue parses and validates user input according to the field type.
//
// Returns:
//   - The parsed value (string, []string, or bool)
//   - A boolean indicating if a value was provided (false for empty input)
//   - An error if validation fails (e.g., invalid enum value, invalid boolean)
func parseInputValue(field SchemaField, input string) (any, bool, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return nil, false, nil
	}

	switch field.Type {
	case FieldTypeString, FieldTypeStringEnum:
		if field.Type == FieldTypeStringEnum && len(field.Options) > 0 {
			if slices.Contains(field.Options, value) {
				return value, true, nil
			}
			return nil, false, fmt.Errorf("invalid value for %s", field.Key)
		}
		return value, true, nil
	case FieldTypeStringList:
		parts := strings.Split(value, ",")
		items := make([]string, 0, len(parts))
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item != "" {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			return nil, false, nil
		}
		return items, true, nil
	case FieldTypeBool:
		parsed, err := strconv.ParseBool(strings.ToLower(value))
		if err != nil {
			return nil, false, fmt.Errorf("invalid boolean for %s", field.Key)
		}
		return parsed, true, nil
	default:
		return nil, false, fmt.Errorf("unsupported field type: %s", field.Type)
	}
}
