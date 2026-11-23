package pipeline

import (
	"strings"
	"testing"
)

// A path may hold a pair of brackets: "./guide(2).md".
//
// The link was cut at the first bracket of the path, and the extension
// of the file was never taken off: the link pointed at
// /<repo>/<section>/guide(2).md while the page is at
// /<repo>/<section>/guide(2)/.
func TestALinkWithBracketsInThePathIsRewritten(t *testing.T) {
	content := "See the [guide](./guide(2).md) for more.\n"

	got := rewriteLinksIterative(content, "repo", "", false, "repo/section/page.md", false)

	if strings.Contains(got, ".md") {
		t.Fatalf("the link keeps the extension of the file: %q", got)
	}
	if !strings.Contains(got, "guide(2)") {
		t.Fatalf("the link lost the brackets of the path: %q", got)
	}
}

// The angle bracket form of a target: "[text](<path with spaces>)".
func TestALinkInTheAngleBracketFormIsRewritten(t *testing.T) {
	content := "See the [my guide](<My Guide.md>) for more.\n"

	got := rewriteLinksIterative(content, "repo", "", false, "repo/section/page.md", false)

	if strings.Contains(got, ".md") {
		t.Fatalf("the link keeps the extension of the file: %q", got)
	}
	if !strings.Contains(got, "<") || !strings.Contains(got, ">") {
		t.Fatalf("the link lost its brackets: %q", got)
	}
}
