package templates

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"
)

// RenderOutputPath renders the output path template string using Go's text/template engine.
//
// The template has access to:
//   - All input values via dot notation (e.g., {{ .Slug }}, {{ .Title }})
//   - The nextInSequence helper function for sequential numbering
//
// Template syntax follows Go's text/template package. Missing keys result in errors
// (missingkey=error option).
//
// Parameters:
//   - pathTemplate: The output path template string (from TemplateMeta.OutputPath)
//   - data: Resolved input values (from ResolveTemplateInputs)
//   - nextSequence: Function to compute next sequence number (can be nil if not used)
//
// Returns:
//   - The rendered output path (relative to docs/, e.g., "adr/adr-001-title.md")
//   - An error if template parsing or execution fails
//
// Example:
//
//	template := "adr/adr-{{ printf \"%03d\" (nextInSequence \"adr\") }}-{{ .Slug }}.md"
//	data := map[string]any{"Slug": "my-decision"}
//	path, err := RenderOutputPath(template, data, sequenceFunc)
//	// Result: "adr/adr-001-my-decision.md"
func RenderOutputPath(pathTemplate string, data map[string]any, nextSequence func(name string) (int, error)) (string, error) {
	funcs := template.FuncMap{
		"nextInSequence": func(name string) (int, error) {
			if nextSequence == nil {
				return 0, errors.New("nextInSequence is not configured")
			}
			return nextSequence(name)
		},
	}

	data = withBuiltinTemplateData(data)

	tpl, err := template.New("output_path").Funcs(funcs).Option("missingkey=error").Parse(pathTemplate)
	if err != nil {
		return "", fmt.Errorf("parse output path template: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render output path template: %w", err)
	}
	return buf.String(), nil
}
