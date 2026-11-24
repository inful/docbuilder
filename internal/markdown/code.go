package markdown

import (
	"sort"
	"strings"

	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// CodeRegion is a range of a markdown document that holds code.
type CodeRegion struct {
	Start int
	Stop  int
}

// CodeBlockRegions returns the code blocks of a markdown document: the
// fenced ones and the indented ones.
//
// The blocks are found with Goldmark, the parser Hugo renders the site
// with, so the regions are the ones the site shows as code. A link or a
// shortcode inside one of them is an example, not a link - ADR-012 keeps
// the links of the code blocks out of the rewriting.
func CodeBlockRegions(body []byte) []CodeRegion {
	source := body
	root := plainTextMarkdown.Parser().Parse(text.NewReader(source), parser.WithContext(parser.NewContext()))

	var regions []CodeRegion
	_ = gmast.Walk(root, func(n gmast.Node, entering bool) (gmast.WalkStatus, error) {
		if !entering {
			return gmast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *gmast.FencedCodeBlock:
			regions = append(regions, lineRegions(node.Lines())...)
		case *gmast.CodeBlock:
			regions = append(regions, lineRegions(node.Lines())...)
		}
		return gmast.WalkContinue, nil
	})

	return sortRegions(regions)
}

// CodeRegions returns the code of a markdown document: the code blocks and
// the inline code spans.
func CodeRegions(body []byte) []CodeRegion {
	source := body
	root := plainTextMarkdown.Parser().Parse(text.NewReader(source), parser.WithContext(parser.NewContext()))

	regions := make([]CodeRegion, 0, 8)
	_ = gmast.Walk(root, func(n gmast.Node, entering bool) (gmast.WalkStatus, error) {
		if !entering {
			return gmast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *gmast.FencedCodeBlock:
			regions = append(regions, lineRegions(node.Lines())...)
		case *gmast.CodeBlock:
			regions = append(regions, lineRegions(node.Lines())...)
		case *gmast.CodeSpan:
			// The parser does not say where an inline code span is, but
			// its text is the content of the span.
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if text, ok := child.(*gmast.Text); ok {
					regions = append(regions, CodeRegion{Start: text.Segment.Start, Stop: text.Segment.Stop})
				}
			}
		}
		return gmast.WalkContinue, nil
	})

	return sortRegions(regions)
}

// lineRegions returns the byte ranges of the lines of a code block.
func lineRegions(lines *text.Segments) []CodeRegion {
	regions := make([]CodeRegion, 0, lines.Len())
	for i := range lines.Len() {
		segment := lines.At(i)
		regions = append(regions, CodeRegion{Start: segment.Start, Stop: segment.Stop})
	}
	return regions
}

// sortRegions sorts the regions by their place in the document and merges
// the ones that overlap.
func sortRegions(regions []CodeRegion) []CodeRegion {
	if len(regions) == 0 {
		return nil
	}
	sort.Slice(regions, func(i, j int) bool { return regions[i].Start < regions[j].Start })

	merged := []CodeRegion{regions[0]}
	for _, region := range regions[1:] {
		last := &merged[len(merged)-1]
		if region.Start <= last.Stop {
			if region.Stop > last.Stop {
				last.Stop = region.Stop
			}
			continue
		}
		merged = append(merged, region)
	}
	return merged
}

// ContainsCode reports whether the offset is inside one of the regions.
func ContainsCode(regions []CodeRegion, offset int) bool {
	for _, region := range regions {
		if region.Start > offset {
			return false
		}
		if offset >= region.Start && offset < region.Stop {
			return true
		}
	}
	return false
}

// ApplyToCode applies fn to the code of the document and leaves the rest of
// it as it is.
func ApplyToCode(content string, regions []CodeRegion, fn func(string) string) string {
	var out strings.Builder
	pos := 0
	for _, region := range regions {
		if region.Start < pos {
			continue
		}
		out.WriteString(content[pos:region.Start])
		out.WriteString(fn(content[region.Start:region.Stop]))
		pos = region.Stop
	}
	out.WriteString(content[pos:])
	return out.String()
}

// ApplyOutsideCode applies fn to the parts of the document that are not
// code, and leaves the code as it is.
func ApplyOutsideCode(content string, regions []CodeRegion, fn func(string) string) string {
	var out strings.Builder
	pos := 0
	for _, region := range regions {
		if region.Start < pos {
			continue
		}
		out.WriteString(fn(content[pos:region.Start]))
		// The code itself is left as it is.
		out.WriteString(content[region.Start:region.Stop])
		pos = region.Stop
	}
	out.WriteString(fn(content[pos:]))
	return out.String()
}
