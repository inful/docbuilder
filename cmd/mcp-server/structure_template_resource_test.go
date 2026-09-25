package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocTemplateStructure_NotEmpty mirrors the same smoke check we do
// for docPageStructureSchema and serverInstructions.
func TestDocTemplateStructure_NotEmpty(t *testing.T) {
	if strings.TrimSpace(docTemplateStructureSchema) == "" {
		t.Fatal("docTemplateStructureSchema must not be empty")
	}
	if len(docTemplateStructureSchema) > 6000 {
		t.Errorf("docTemplateStructureSchema is %d bytes; keep under 6000", len(docTemplateStructureSchema))
	}
}

// TestDocTemplateStructure_ContainsRequiredFields locks the resource to
// a minimum viable schema.
func TestDocTemplateStructure_ContainsRequiredFields(t *testing.T) {
	required := []string{
		"## Three Parts of a Template Document",
		"## Required Template Frontmatter",
		"## Optional Template Frontmatter",
		"## Schema Field Types",
		"## Output Path Template Variables",
		"## Sequence Configuration",
		"## Body Block Rules",
		"params.docbuilder.template",
		"type", "name", "output_path",
		"schema", "defaults", "sequence",
		"string", "string_enum", "string_list", "bool",
		"nextInSequence",
	}
	for _, want := range required {
		if !strings.Contains(docTemplateStructureSchema, want) {
			t.Errorf("docTemplateStructureSchema missing %q", want)
		}
	}
}

// TestDocTemplateStructure_StaysInSyncWithDoc catches drift between the
// const (exposed via structure://template) and the human-readable file
// at docs/reference/doc-template-structure.md. The contract is:
//
//   - Every first-column row in the doc's "Required Template Frontmatter"
//     table must appear in the const.
//   - Every first-column row in the "Optional Template Frontmatter" table
//     must appear in the const.
//   - Every field type listed in the doc's "Schema Field Types" table must
//     appear in the const.
//   - Section headings in the const must exist in the doc.
func TestDocTemplateStructure_StaysInSyncWithDoc(t *testing.T) {
	docPath, err := findDocTemplateStructureFile()
	if err != nil {
		t.Skipf("could not locate docs/reference/doc-template-structure.md (running outside repo?): %v", err)
	}
	body, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	doc := string(body)

	// Required frontmatter table.
	requiredSection := sectionBetween(doc, "### Required fields", "### Optional fields")
	for _, field := range tableFirstColumn(requiredSection) {
		if !fieldPresent(docTemplateStructureSchema, field) {
			t.Errorf("const missing required field %q from %s", field, docPath)
		}
	}

	// Optional frontmatter table.
	optionalSection := sectionBetween(doc, "### Optional fields", "## Schema fields")
	for _, field := range tableFirstColumn(optionalSection) {
		if !fieldPresent(docTemplateStructureSchema, field) {
			t.Errorf("const missing optional field %q from %s", field, docPath)
		}
	}

	// Field types — they appear in the const as a single-column "Type" table
	// under "## Schema Field Types". The doc has them in the "### Field types"
	// table. We extract the doc's first-column entries and confirm each
	// appears in the const.
	fieldTypesSection := sectionBetween(doc, "### Field types", "### Glob suggestions")
	for _, fieldType := range tableFirstColumn(fieldTypesSection) {
		if !strings.Contains(docTemplateStructureSchema, fieldType) {
			t.Errorf("const missing schema field type %q from %s", fieldType, docPath)
		}
	}

	// Section headings: spot-check a few.
	for _, heading := range []string{
		"Three Parts of a Template Document",
		"Required Template Frontmatter",
		"Optional Template Frontmatter",
		"Schema Field Types",
		"Output Path Template Variables",
		"Sequence Configuration",
		"Body Block Rules",
	} {
		if !strings.Contains(docTemplateStructureSchema, heading) {
			t.Errorf("const missing heading %q", heading)
		}
	}
}

// findDocTemplateStructureFile mirrors findDocPageStructureFile but for
// the templates schema doc.
func findDocTemplateStructureFile() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "docs", "reference", "doc-template-structure.md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}
