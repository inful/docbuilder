package markdown

import "testing"

// The text of a page must not carry the markup around it: the reader
// searches for words and reads excerpts.
func TestPlainTextDropsTheMarkup(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"plain text stays", "Just words.", "Just words."},
		{"bold loses its asterisks", "a **bold** word", "a bold word"},
		{"italic loses its asterisks", "an *italic* word", "an italic word"},
		{"a code span loses its backticks", "a `code` word", "a code word"},
		{"a link keeps its text", "a [link](https://example.com) here", "a link here"},
		{"an image keeps its alt text", "![a picture](img.png)", "a picture"},
		{"a heading loses its hashes", "# A heading", "A heading"},
		{"a list loses its dashes", "- one\n- two", "one two"},
		{"a quote loses its marker", "> quoted", "quoted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PlainText([]byte(tc.body))
			if got != tc.want {
				t.Fatalf("PlainText(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

// The words of a code block are words of the page: a reader looking
// for them wants to find the page they are on.
func TestPlainTextKeepsTheWordsOfACodeBlock(t *testing.T) {
	got := PlainText([]byte("Run this:\n\n```bash\ndocbuilder install\n```\n"))
	want := "Run this: docbuilder install"
	if got != want {
		t.Fatalf("PlainText = %q, want %q", got, want)
	}
}

// The markup of the page must not leak into the text: a reader sees
// the words, not the source.
func TestPlainTextKeepsNoMarkup(t *testing.T) {
	body := "# Title\n\nSome **bold** and `code` with a [link](https://example.com).\n\n```bash\ndocbuilder install\n```\n"
	got := PlainText([]byte(body))
	for _, forbidden := range []string{"**", "`", "](", "```", "#"} {
		if contains(got, forbidden) {
			t.Fatalf("PlainText = %q, it still holds %q", got, forbidden)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}
