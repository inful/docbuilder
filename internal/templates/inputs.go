package templates

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// FieldType defines the supported input field types.
type FieldType string

const (
	FieldTypeString     FieldType = "string"
	FieldTypeStringEnum FieldType = "string_enum"
	FieldTypeStringList FieldType = "string_list"
	FieldTypeBool       FieldType = "bool"
)

// SchemaField represents a single prompt field in the template schema.
type SchemaField struct {
	Key      string    `json:"key"`
	Type     FieldType `json:"type"`
	Required bool      `json:"required"`
	Options  []string  `json:"options,omitempty"`
}

// TemplateSchema describes the fields required to instantiate a template.
type TemplateSchema struct {
	Fields []SchemaField `json:"fields"`
}

// Prompter provides responses for template fields.
type Prompter interface {
	Prompt(field SchemaField) (string, error)
}

// ResolveTemplateInputs merges defaults, overrides, and prompt responses.
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
