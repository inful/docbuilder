package markdown

import "testing"

// The server-side index must hold the same words as the theme's own
// index: the theme holds the text of the rendered page. A reader who
// searches finds the same pages either way.
func TestPlainTextMatchesTheTheme(t *testing.T) {
	body := "Some **bold** and *italic* and `code` text with a [link](https://example.com).\n\n" +
		"| a | b |\n|---|---|\n| 1 | 2 |\n\n" +
		"- [ ] not done\n- [x] done\n\n" +
		"~~struck~~ text.\n\n" +
		"Term\n:   Definition\n"
	want := "Some bold and italic and code text with a link. " +
		"a b 1 2 " +
		"not done " + "done " +
		"struck text. " +
		"Term Definition"

	if got := PlainText([]byte(body)); got != want {
		t.Fatalf("PlainText =\n%q\nwant\n%q", got, want)
	}
}
