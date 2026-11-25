package pipeline

import (
	"strings"
	"testing"
)

// The styles of the code blocks come from the theme: the theme loads
// them through its own stylesheet with Hugo's meta.load-css, and the
// stylesheet we ship is built without them. The code blocks of every
// site had no colors, and their line numbers stood in a column of
// their own.
func TestTheVendoredSyntaxStylesAreShipped(t *testing.T) {
	css := string(docsyChromaCSS)

	if !strings.Contains(css, ".chroma .k") {
		t.Error("the vendored syntax styles do not hold the styles of the keywords")
	}
	if !strings.Contains(css, ".chroma .lnt") {
		t.Error("the vendored syntax styles do not hold the styles of the line numbers")
	}
	if !strings.Contains(css, "[data-bs-theme='light']") ||
		!strings.Contains(css, "[data-bs-theme='dark']") {
		t.Error("the vendored syntax styles do not follow the light and the dark mode of the theme")
	}
}
