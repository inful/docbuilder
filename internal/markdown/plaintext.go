package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// PlainText turns a Markdown body into the text of the page.
//
// The words of the page survive; the markup around them does not. A
// bold word loses its asterisks, a code span loses its backticks, a
// code block keeps its content but loses its fences, and a link keeps
// its text but loses its address.
//
// The text of a code block is kept on purpose: the words of a code
// block are words of the page, and a reader looking for them wants to
// find the page they are on.
func PlainText(body []byte) string {
	md := goldmark.New()
	source := body
	root := md.Parser().Parse(text.NewReader(source), parser.WithContext(parser.NewContext()))

	var parts []string
	_ = gmast.Walk(root, func(n gmast.Node, entering bool) (gmast.WalkStatus, error) {
		if !entering {
			return gmast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *gmast.Text:
			// A raw text node is a code span: its content is the
			// code, and it is text of the page.
			parts = append(parts, string(node.Segment.Value(source)))
		case *gmast.String:
			parts = append(parts, string(node.Value))
		case *gmast.FencedCodeBlock:
			parts = append(parts, codeLines(node, source))
		case *gmast.CodeBlock:
			parts = append(parts, codeLines(node, source))
		}
		return gmast.WalkContinue, nil
	})

	// The parts come from the markup, so the spaces around them are
	// markup too: "a **bold** word" is three parts with a space
	// before and after the bold one.
	kept := parts[:0]
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " ")
}

// codeLines returns the content of a code block.
func codeLines(node gmast.Node, source []byte) string {
	lines := node.Lines()
	var out strings.Builder
	for i := range lines.Len() {
		line := lines.At(i)
		out.Write(line.Value(source))
	}
	return out.String()
}
