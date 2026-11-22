package markdown

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// plainTextMarkdown parses Markdown the way the site does.
//
// The extensions are the ones the theme's own index gets for free: the
// index of a site that uses the browser-side search holds the text of
// the rendered page, and the server-side index must hold the same
// words.
var plainTextMarkdown = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
		extension.Linkify,
		extension.Footnote,
		extension.DefinitionList,
	),
)

// piece is one run of text of the page, with the place it came from in
// the source.
type piece struct {
	text  string
	start int
	stop  int
}

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
	source := body
	root := plainTextMarkdown.Parser().Parse(text.NewReader(source), parser.WithContext(parser.NewContext()))

	var pieces []piece
	_ = gmast.Walk(root, func(n gmast.Node, entering bool) (gmast.WalkStatus, error) {
		if !entering {
			return gmast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *gmast.Text:
			// A raw text node is a code span: its content is the
			// code, and it is text of the page.
			segment := node.Segment
			pieces = append(pieces, piece{
				text:  string(segment.Value(source)),
				start: segment.Start,
				stop:  segment.Stop,
			})
		case *gmast.String:
			pieces = append(pieces, piece{text: string(node.Value), start: -1, stop: -1})
		case *gmast.FencedCodeBlock:
			if p, ok := codePiece(node, source); ok {
				pieces = append(pieces, p)
			}
		case *gmast.CodeBlock:
			if p, ok := codePiece(node, source); ok {
				pieces = append(pieces, p)
			}
		}
		return gmast.WalkContinue, nil
	})

	return join(pieces, source)
}

// codePiece returns the content of a code block as one piece.
func codePiece(node gmast.Node, source []byte) (piece, bool) {
	lines := node.Lines()
	if lines.Len() == 0 {
		return piece{}, false
	}
	var text strings.Builder
	start, stop := -1, -1
	for i := range lines.Len() {
		line := lines.At(i)
		if start < 0 {
			start = line.Start
		}
		stop = line.Stop
		text.Write(line.Value(source))
	}
	return piece{text: text.String(), start: start, stop: stop}, true
}

// join puts the pieces of the page together.
//
// Two pieces that stand next to each other in the source are joined
// without a space: the space is already in the text ("a " and "link"
// make "a link"). Two pieces that stand apart - two blocks, or two
// words with markup between them - get a space, so the words do not
// run into each other.
func join(pieces []piece, source []byte) string {
	var out strings.Builder
	previous := piece{stop: -1}
	for _, p := range pieces {
		if strings.TrimSpace(p.text) == "" {
			continue
		}
		if out.Len() > 0 {
			out.WriteString(separator(previous, p, source))
		}
		out.WriteString(p.text)
		previous = p
	}
	return strings.TrimSpace(out.String())
}

// separator is what goes between two pieces of the page.
//
// The space between two words is in the text itself ("a " and "link"
// make "a link"). The markup between them is not: "**bold**" is the
// word "bold" next to the word before it. So the space goes between
// the pieces only when the source has whitespace between them.
func separator(previous, next piece, source []byte) string {
	if previous.stop < 0 || next.start < 0 || next.start < previous.stop {
		// The place of the piece in the source is not known, so the
		// words are kept apart.
		return " "
	}
	gap := source[previous.stop:next.start]
	if bytes.ContainsAny(gap, " \t\n\r") {
		return " "
	}
	return ""
}
