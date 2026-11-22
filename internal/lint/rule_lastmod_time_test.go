package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// A date without a time is read by the YAML parser as a time. The rule
// asks for the day, so the value is not a malformed date.
func TestLastmodAcceptsADateWithoutATime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	content := "---\n" +
		"title: Doc\n" +
		"date: 2026-01-01\n" +
		"lastmod: 2026-01-01\n" +
		"categories:\n  - how-to\n" +
		"tags:\n  - one\n" +
		"uid: 11111111-1111-1111-1111-111111111111\n" +
		"fingerprint: abc\n" +
		"---\n\n# Doc\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the doc: %v", err)
	}

	rule := &FrontmatterRequiredFieldsRule{}
	issues, err := rule.Check(path)
	if err != nil {
		t.Fatalf("check the doc: %v", err)
	}

	for _, issue := range issues {
		if issue.Rule == rule.Name() {
			t.Errorf("the rule reported %q", issue.Message)
		}
	}
}
