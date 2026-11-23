package service

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/templates"
)

// The path of the document and the uid in its body are the same.
//
// The builtins were resolved by every render: the path and the body
// each had their own uuid, and the body referenced a document that does
// not exist.
func TestThePathAndTheBodyHaveTheSameUID(t *testing.T) {
	page := &templates.TemplatePage{
		Meta: templates.TemplateMeta{
			OutputPath: "adr/adr-{{ .UID }}.md",
			Type:       "adr",
		},
		Body: "# ADR\n\nuid: \"{{ .UID }}\"\n",
	}

	svc := NewTemplateService()
	outputPath, body, err := svc.RenderPreview(page, map[string]any{}, nil)
	if err != nil {
		t.Fatalf("render the preview: %v", err)
	}

	uid := strings.TrimSuffix(strings.TrimPrefix(outputPath, "adr/adr-"), ".md")
	if uid == "" {
		t.Fatalf("the path has no uid: %q", outputPath)
	}
	if !strings.Contains(body, uid) {
		t.Fatalf("the body has another uid than the path: %q", body)
	}
}
