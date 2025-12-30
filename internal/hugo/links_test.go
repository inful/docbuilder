package hugo

import "testing"

func TestRewriteRelativeMarkdownLinks(t *testing.T) {
	cases := []struct{ in, want string }{
		{"See [Doc](foo.md) for details", "See [Doc](../foo/) for details"}, // same-dir needs ../
		{"[Anchor](bar.md#sec)", "[Anchor](../bar/#sec)"},                   // same-dir with anchor
		{"Absolute [Link](https://example.com/file.md)", "Absolute [Link](https://example.com/file.md)"},
		{"Image ref ![Alt](img.png)", "Image ref ![Alt](img.png)"}, // images unaffected
		{"Mail [Me](mailto:test@example.com)", "Mail [Me](mailto:test@example.com)"},
		{"Hash [Ref](#local)", "Hash [Ref](#local)"},
		{"Nested ./path [Ref](./sub/thing.md)", "Nested ./path [Ref](../sub/thing/)"},     // ./ removed for regular page, then ../ added
		{"Up one [Ref](../other.md)", "Up one [Ref](../../other/)"},                       // Hugo needs extra ../
		{"Markdown long ext [Ref](guide.markdown)", "Markdown long ext [Ref](../guide/)"}, // same-dir needs ../
		{"Child dir [Ref](guide/setup.md)", "Child dir [Ref](../guide/setup/)"},           // subdirectory also needs ../
	}
	for i, c := range cases {
		got := RewriteRelativeMarkdownLinks(c.in, "", "", false) // false = not an index page
		if got != c.want {
			t.Errorf("case %d: got %q want %q", i, got, c.want)
		}
	}
}

func TestRewriteRelativeMarkdownLinks_RepositoryRootRelative(t *testing.T) {
	cases := []struct {
		in, repo, forge, want string
	}{
		{
			"See [Doc](/api/reference.md) for details",
			"my-project", "",
			"See [Doc](/my-project/api/reference/) for details",
		},
		{
			"[Guide](/how-to/authentication.md)",
			"franklin-api", "",
			"[Guide](/franklin-api/how-to/authentication/)",
		},
		{
			"[Guide](/how-to/authentication.md#setup)",
			"franklin-api", "",
			"[Guide](/franklin-api/how-to/authentication/#setup)",
		},
		{
			"[Doc](/api/reference.md)",
			"my-repo", "github",
			"[Doc](/github/my-repo/api/reference/)",
		},
		{
			"Mixed [repo-root](/api/guide.md) and [relative](../other.md)",
			"my-project", "",
			"Mixed [repo-root](/my-project/api/guide/) and [relative](../../other/)",
		},
	}
	for i, c := range cases {
		got := RewriteRelativeMarkdownLinks(c.in, c.repo, c.forge, false) // false = not an index page
		if got != c.want {
			t.Errorf("case %d: got %q want %q", i, got, c.want)
		}
	}
}

func TestRewriteRelativeMarkdownLinks_IndexPages(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Index pages (_index.md or README.md) are at section root, so relative links don't need extra ../
		{"See [Doc](foo.md) for details", "See [Doc](foo/) for details"},
		{"[Anchor](bar.md#sec)", "[Anchor](bar/#sec)"},
		{"Subdirectory [Link](how-to/authentication.md)", "Subdirectory [Link](how-to/authentication/)"},
		{"Up one [Ref](../other.md)", "Up one [Ref](../other/)"}, // No extra ../ for index pages
		{"Two up [Ref](../../another.md)", "Two up [Ref](../../another/)"},
	}
	for i, c := range cases {
		got := RewriteRelativeMarkdownLinks(c.in, "", "", true) // true = index page
		if got != c.want {
			t.Errorf("index page case %d: got %q want %q", i, got, c.want)
		}
	}
}
