package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A document without frontmatter has no frontmatter. The message said
// that the field "frontmatter" was missing, and the explanation asked
// for a "frontmatter" field in the frontmatter.
func TestTheMessageOfADocumentWithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "page.md")
	if err := os.WriteFile(doc, []byte("# Page\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("write the document: %v", err)
	}

	l := NewLinter(&Config{Format: formatText})
	res, err := l.LintPath(dir)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}

	for _, issue := range res.Issues {
		if issue.Rule != ruleFrontmatterRequiredFields {
			continue
		}
		if strings.Contains(issue.Message, "frontmatter field: frontmatter") {
			t.Fatalf("the message names a field that is not one: %s", issue.Message)
		}
	}
}
