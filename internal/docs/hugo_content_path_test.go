package docs

import "testing"

// A file called index.png is published as index.png. The rename to
// _index belongs to the markdown files: an asset that was renamed was
// published under a name that no link points at.
func TestTheAssetsKeepTheirName(t *testing.T) {
	got := HugoContentPath("", "repo", "guide", "index", ".png", false)
	want := "content/repo/guide/index.png"
	if got != want {
		t.Fatalf("the path of the asset is %q, want %q", got, want)
	}
}

// The markdown files keep the name of the section index.
func TestTheMarkdownIndexKeepsItsName(t *testing.T) {
	got := HugoContentPath("", "repo", "guide", "index", ".md", false)
	want := "content/repo/guide/_index.md"
	if got != want {
		t.Fatalf("the path of the index is %q, want %q", got, want)
	}
}
