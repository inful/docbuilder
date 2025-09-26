package hugo

import "testing"

func TestRewriteRelativeMarkdownLinks(t *testing.T) {
    cases := []struct{ in, want string }{
        {"See [Doc](foo.md) for details", "See [Doc](foo) for details"},
        {"[Anchor](bar.md#sec)", "[Anchor](bar#sec)"},
        {"Absolute [Link](https://example.com/file.md)", "Absolute [Link](https://example.com/file.md)"},
        {"Image ref ![Alt](img.png)", "Image ref ![Alt](img.png)"}, // images unaffected
        {"Mail [Me](mailto:test@example.com)", "Mail [Me](mailto:test@example.com)"},
        {"Hash [Ref](#local)", "Hash [Ref](#local)"},
        {"Nested ./path [Ref](./sub/thing.md)", "Nested ./path [Ref](./sub/thing)"},
        {"Up one [Ref](../other.md)", "Up one [Ref](../other)"},
        {"Markdown long ext [Ref](guide.markdown)", "Markdown long ext [Ref](guide)"},
    }
    for i, c := range cases {
        got := RewriteRelativeMarkdownLinks(c.in)
        if got != c.want {
            t.Errorf("case %d: got %q want %q", i, got, c.want)
        }
    }
}
