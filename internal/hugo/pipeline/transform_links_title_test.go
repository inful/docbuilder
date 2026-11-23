package pipeline

import "testing"

// A link may carry a title: "[text](guide.md "Title")". The path is
// rewritten and the title is kept.
func TestALinkWithATitleIsRewritten(t *testing.T) {
	content := "See the [guide](Guide.md \"The guide\") for more.\n"

	got := rewriteLinksIterative(content, "repo", "", false, "repo/section/page.md", false)

	if got == content {
		t.Fatalf("the link with a title was not rewritten: %q", got)
	}
	if !containsString(got, "\"The guide\"") {
		t.Fatalf("the title of the link was lost: %q", got)
	}
}
