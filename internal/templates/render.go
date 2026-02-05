package templates

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"
)

// RenderTemplateBody renders a template body using Go's text/template engine.
//
// The template has access to:
//   - All input values via dot notation (e.g., {{ .Title }}, {{ .Slug }})
//   - The nextInSequence helper function for sequential numbering
//
// Template syntax follows Go's text/template package. Missing keys result in errors
// (missingkey=error option).
//
// Parameters:
//   - bodyTemplate: The markdown template string (from TemplatePage.Body)
//   - data: Resolved input values (from ResolveTemplateInputs)
//   - nextSequence: Function to compute next sequence number (can be nil if not used)
//
// Returns:
//   - The rendered markdown content
//   - An error if template parsing or execution fails
//
// Example:
//
//	template := "# {{ .Title }}\n\nSlug: {{ .Slug }}"
//	data := map[string]any{"Title": "My Doc", "Slug": "my-doc"}
//	rendered, err := RenderTemplateBody(template, data, nil)
//	// Result: "# My Doc\n\nSlug: my-doc"
func RenderTemplateBody(bodyTemplate string, data map[string]any, nextSequence func(name string) (int, error)) (string, error) {
	funcs := template.FuncMap{
		"nextInSequence": func(name string) (int, error) {
			if nextSequence == nil {
				return 0, errors.New("nextInSequence is not configured")
			}
			return nextSequence(name)
		},
	}

	data = withBuiltinTemplateData(data)

	tpl, err := template.New("body").Funcs(funcs).Option("missingkey=error").Parse(bodyTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template body: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template body: %w", err)
	}
	return buf.String(), nil
}
