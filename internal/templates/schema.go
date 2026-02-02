package templates

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseTemplateSchema parses the schema JSON from template metadata.
func ParseTemplateSchema(raw string) (TemplateSchema, error) {
	if strings.TrimSpace(raw) == "" {
		return TemplateSchema{}, nil
	}

	var schema TemplateSchema
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		return TemplateSchema{}, fmt.Errorf("parse template schema: %w", err)
	}
	return schema, nil
}

// ParseTemplateDefaults parses the defaults JSON from template metadata.
func ParseTemplateDefaults(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var defaults map[string]any
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		return nil, fmt.Errorf("parse template defaults: %w", err)
	}
	return defaults, nil
}
