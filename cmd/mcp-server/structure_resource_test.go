package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocPageStructure_NotEmpty mirrors the same smoke check we do for
// serverInstructions: the const must be non-empty and not bloated.
func TestDocPageStructure_NotEmpty(t *testing.T) {
	if strings.TrimSpace(docPageStructureSchema) == "" {
		t.Fatal("docPageStructureSchema must not be empty")
	}
	// Generous cap — the resource text is meant to be LLM-readable, not
	// a full Markdown doc. If it grows past this, the doc page should
	// move to the resource table and we should serve it via a URI read.
	if len(docPageStructureSchema) > 6000 {
		t.Errorf("docPageStructureSchema is %d bytes; keep under 6000 — full content belongs in docs/reference/doc-page-structure.md",
			len(docPageStructureSchema))
	}
}

// TestDocPageStructure_ContainsRequiredFields locks the resource to a
// minimum viable schema. If you intentionally add or remove a required
// field, update both the const and docs/reference/doc-page-structure.md
// in the same commit (TestDocPageStructure_StaysInSyncWithDoc covers the
// cross-check).
func TestDocPageStructure_ContainsRequiredFields(t *testing.T) {
	required := []string{
		"## Required Frontmatter",
		"## Optional Frontmatter",
		"## Body Rules",
		"## Filename Rules",
		"## Verification Workflow",
		"title", "uid", "date", "lastmod", "fingerprint",
		"categories", "tags", "aliases",
		"weight", "draft", "description",
	}
	for _, want := range required {
		if !strings.Contains(docPageStructureSchema, want) {
			t.Errorf("docPageStructureSchema missing %q", want)
		}
	}
}

// TestDocPageStructure_StaysInSyncWithDoc catches drift between the const
// (exposed via structure://doc-page) and the human-readable file at
// docs/reference/doc-page-structure.md. The contract is:
//
//   - Every required frontmatter field name listed in the file's required
//     table must appear in the const.
//   - Every optional frontmatter field name listed in the file must appear
//     in the const.
//   - Every body-rule keyword in the file's body-rules section must appear
//     in the const.
//   - Every filename-rule keyword in the file must appear in the const.
//
// The file is the source of truth for wording; the const is the source
// of truth for what the LLM sees. If you update one, update the other.
func TestDocPageStructure_StaysInSyncWithDoc(t *testing.T) {
	docPath, err := findDocPageStructureFile()
	if err != nil {
		t.Skipf("could not locate docs/reference/doc-page-structure.md (running outside repo?): %v", err)
	}
	body, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	doc := string(body)

	// Required fields: first column of every row in the "Required Frontmatter"
	// table in the doc must appear as a row in the const's "Required
	// Frontmatter" table.
	requiredSection := sectionBetween(doc, "## Required Frontmatter", "## Optional Frontmatter")
	for _, field := range tableFirstColumn(requiredSection) {
		if !fieldPresent(docPageStructureSchema, field) {
			t.Errorf("const missing required field %q from %s", field, docPath)
		}
	}

	optionalSection := sectionBetween(doc, "## Optional Frontmatter", "## Build-Injected")
	for _, field := range tableFirstColumn(optionalSection) {
		if !fieldPresent(docPageStructureSchema, field) {
			t.Errorf("const missing optional field %q from %s", field, docPath)
		}
	}

	// Spot-check the file's section headings exist in the const.
	for _, heading := range []string{
		"Required Frontmatter",
		"Optional Frontmatter",
		"Body Rules",
		"Filename Rules",
	} {
		if !strings.Contains(docPageStructureSchema, heading) {
			t.Errorf("const missing heading %q", heading)
		}
	}
}

// findDocPageStructureFile walks up from the test working directory to
// locate docs/reference/doc-page-structure.md. We don't import the
// repo's path helpers to keep this test self-contained.
func findDocPageStructureFile() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "docs", "reference", "doc-page-structure.md")
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

// sectionBetween returns the substring of body from the start of the
// line beginning with `start` to the start of the line beginning with
// `end`. If either marker is missing, returns "".
func sectionBetween(body, start, end string) string {
	i := strings.Index(body, start)
	if i < 0 {
		return ""
	}
	j := strings.Index(body[i+len(start):], end)
	if j < 0 {
		return body[i:]
	}
	return body[i : i+len(start)+j]
}

// tableFirstColumn returns the first `|`-delimited cell of every
// Markdown table row in s. Returns nothing for separator rows like
// |---|---|, header rows like | Field | Type |, or rows outside tables.
//
// We use this rather than scanning arbitrary backticks because the doc
// has lots of inline-code references that aren't field names
// (e.g., 'uuidgen', 'how-to', 'true').
func tableFirstColumn(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		// cells[0] is empty (line starts with |), cells[1] is the first
		// column, last cell is empty (line ends with |). After trimming
		// we want cells[1].
		if len(cells) < 3 {
			continue
		}
		first := strings.TrimSpace(cells[1])
		if first == "" || !isLikelyFieldName(first) {
			continue
		}
		// Strip surrounding backticks if the cell is code-formatted.
		first = strings.TrimPrefix(first, "`")
		first = strings.TrimSuffix(first, "`")
		if !isLikelyFieldName(first) {
			continue
		}
		out = append(out, first)
	}
	return out
}

// fieldPresent returns true if `field` appears as a first-column entry in
// any Markdown table row inside `body`. We use this to confirm the const
// hasn't drifted from the doc's tables.
func fieldPresent(body, field string) bool {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		first := strings.TrimSpace(cells[1])
		first = strings.TrimPrefix(first, "`")
		first = strings.TrimSuffix(first, "`")
		if first == field {
			return true
		}
	}
	return false
}

func isLikelyFieldName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}
