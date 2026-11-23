package pipeline

import (
	"strings"
	"testing"
)

// A sibling README is the index of the section: the pipeline renames it
// to _index, and the page is published at the directory of the document.
//
// The rewrite kept the name of the file, so the link pointed at
// /<repo>/<section>/readme while the page is at /<repo>/<section>/.
func TestASiblingReadmeLinkPointsAtTheSection(t *testing.T) {
	content := "See the [setup](README.md) for more.\n"

	got := rewriteLinksIterative(content, "repo", "", false, "repo/section/page.md", false)

	if !strings.Contains(got, "](") {
		t.Fatalf("the link was not rewritten: %q", got)
	}
	if strings.Contains(got, "readme") {
		t.Fatalf("the link keeps the name of the file: %q", got)
	}
	if !strings.Contains(got, "repo/section/") {
		t.Fatalf("the link does not point at the section: %q", got)
	}
}

// A sibling index is the index of the section as well.
func TestASiblingIndexLinkPointsAtTheSection(t *testing.T) {
	content := "See the [setup](index.md) for more.\n"

	got := rewriteLinksIterative(content, "repo", "", false, "repo/section/page.md", false)

	if strings.Contains(got, "index") {
		t.Fatalf("the link keeps the name of the file: %q", got)
	}
	if !strings.Contains(got, "repo/section/") {
		t.Fatalf("the link does not point at the section: %q", got)
	}
}
