package frontmatter

import (
	"testing"
)

// The closing delimiter can be the last line of the file, without a
// newline behind it. The document is valid, and the linter said it had
// no frontmatter at all.
func TestTheClosingDelimiterAtTheEndOfTheFileIsFound(t *testing.T) {
	content := []byte("---\ntitle: Only FM\ncategories:\n  - reference\n---")

	fm, body, had, _, err := Split(content)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if !had {
		t.Fatal("the frontmatter was not found")
	}
	if len(body) != 0 {
		t.Fatalf("the body is %q, want nothing", body)
	}
	if string(fm) != "title: Only FM\ncategories:\n  - reference\n" {
		t.Fatalf("the frontmatter is %q", fm)
	}
}
