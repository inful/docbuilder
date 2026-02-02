package templates

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"
)

// RenderOutputPath renders the template output path using provided data.
func RenderOutputPath(pathTemplate string, data map[string]any, nextSequence func(name string) (int, error)) (string, error) {
	funcs := template.FuncMap{
		"nextInSequence": func(name string) (int, error) {
			if nextSequence == nil {
				return 0, errors.New("nextInSequence is not configured")
			}
			return nextSequence(name)
		},
	}

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
