package pipeline

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// The path of a link is cut by the characters of its suffix.
//
// The lowercase of a character can have another length ("İ" is two
// bytes and "i" is one): the path was cut at the byte of the lowercased
// path, and the link lost a character of its path.
func TestThePathOfALinkIsCutByTheSuffix(t *testing.T) {
	content := "See the [guide](./İx/index.md) for more.\n"

	got := rewriteLinksIterative(content, "repo", "", false, "repo/section/page.md", false)

	if !utf8.ValidString(got) {
		t.Fatalf("the link was cut in the middle of a character: %q", got)
	}
	if strings.Contains(got, ".md") {
		t.Fatalf("the link keeps the extension of the file: %q", got)
	}
	if !strings.Contains(got, "repo/section/") {
		t.Fatalf("the link does not point at the section: %q", got)
	}
	// The address is lowercased, and the "x" of the path is there.
	if !strings.Contains(got, "ix/") {
		t.Fatalf("the link lost a character of its path: %q", got)
	}
}
