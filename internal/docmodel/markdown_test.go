package docmodel

import "testing"

// TestIsMarkdownFile exercises every case the three prior predicates
// disagreed on. The canonical answer (.md / .markdown / .mdown / .mkd
// in any case + a small set of negative inputs) is what discovery,
// lint, and the pipeline will all see going forward.
func TestIsMarkdownFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// Positives on each recognized extension.
		{name: "lowercase .md", path: "docs/readme.md", want: true},
		{name: "uppercase .MD", path: "docs/README.MD", want: true},
		{name: "mixed case .Md", path: "docs/ReadMe.Md", want: true},
		{name: "lowercase .markdown", path: "docs/page.markdown", want: true},
		{name: "uppercase .MARKDOWN", path: "docs/page.MARKDOWN", want: true},
		{name: ".mdown", path: "docs/page.mdown", want: true},
		{name: ".mkd", path: "docs/page.mkd", want: true},

		// Negatives.
		{name: "empty", path: "", want: false},
		{name: "no extension", path: "docs/readme", want: false},
		{name: "different extension", path: "docs/page.html", want: false},
		{name: "hidden dotfile", path: ".README.md", want: true},
		{name: "fragment in path is opaque to extension lookup", path: "docs/page.md#section", want: false},
		{name: "query in path is opaque", path: "docs/page.md?ref=main", want: false},
		// Go's filepath.Ext reports the dot before MD as the separator,
		// so .MD (hidden file) is still a ".MD"-suffixed path. Keep this
		// visible in the test so future refactors don't silently change it.
		{name: "hidden .MD counts", path: "docs/.MD", want: true},
		{name: "double extension .MD.md", path: "docs/.MD.md", want: true},

		// path-only basename matters, so a leading directory is irrelevant.
		{name: "absolute / lowercase", path: "/tmp/dir/x.md", want: true},
		{name: "absolute / uppercase", path: "/tmp/dir/x.MD", want: true},
		// literal '#' inside the basename is preserved (filepath.Ext
		// works from the right and ignores query/fragment conventions).
		{name: "literal hash in filename", path: "tutorial#1.md", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMarkdownFile(tt.path); got != tt.want {
				t.Errorf("IsMarkdownFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestStripMarkdownExt covers both extensions and non-matches. The
// casing of the body must be preserved.
func TestStripMarkdownExt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "lowercase .md", in: "docs/readme.md", want: "docs/readme"},
		{name: "uppercase .MD", in: "docs/README.MD", want: "docs/README"},
		{name: "lowercase .markdown", in: "docs/page.markdown", want: "docs/page"},
		{name: "uppercase .MARKDOWN", in: "docs/page.MARKDOWN", want: "docs/page"},
		{name: "no extension", in: "docs/readme", want: "docs/readme"},
		{name: "different extension", in: "docs/page.html", want: "docs/page.html"},
		{name: "empty", in: "", want: ""},
		{name: "just .md", in: ".md", want: ""},
		{name: "just .markdown", in: ".markdown", want: ""},
		// .mdown / .mkd are recognized by IsMarkdownFile but not
		// stripped (no caller strips them today).
		{name: ".mdown not stripped", in: "page.mdown", want: "page.mdown"},
		{name: ".mkd not stripped", in: "page.mkd", want: "page.mkd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripMarkdownExt(tt.in); got != tt.want {
				t.Errorf("StripMarkdownExt(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
