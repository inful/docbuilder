package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// A link inside a code block is an example, not a link in the document.
// The how-to guides show what a site-rooted link looks like, and the
// rule must not flag their own examples.
func TestInternalLinkStyleRule_SkipsCodeBlocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")

	content := "---\n" +
		"title: Test\n" +
		"---\n" +
		"\n" +
		"A real [link](/api/reference.md).\n" +
		"\n" +
		"```markdown\n" +
		"[example](/api/reference.md)\n" +
		"```\n" +
		"\n" +
		"An inline example `![Hero](/Static/Hero.PNG)` is not a link either.\n" +
		"\n" +
		"Another real [link](/how-to/setup.md).\n"

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	issues, err := (&InternalLinkStyleRule{}).Check(path)
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2 (the two real links): %+v", len(issues), issues)
	}
	for _, issue := range issues {
		if issue.Line == 9 {
			t.Errorf("the link inside the code block was flagged: %+v", issue)
		}
	}
}
