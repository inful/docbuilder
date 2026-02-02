package templates

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"
)

// RenderTemplateBody renders the template body with provided data and helpers.
func RenderTemplateBody(bodyTemplate string, data map[string]any, nextSequence func(name string) (int, error)) (string, error) {
	funcs := template.FuncMap{
		"nextInSequence": func(name string) (int, error) {
			if nextSequence == nil {
				return 0, errors.New("nextInSequence is not configured")
			}
			return nextSequence(name)
		},
	}

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
