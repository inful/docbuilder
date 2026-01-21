package markdown

import (
	"sort"

	"github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// ParseBody parses a Markdown body (frontmatter already removed) into a Goldmark AST.
func ParseBody(body []byte, _ Options) (gmast.Node, error) {
	md := goldmark.New()
	root := md.Parser().Parse(text.NewReader(body))
	return root, nil
}

// ExtractLinks parses a Markdown body and extracts link-like constructs.
//
// This is an analysis API; it does not attempt to re-render Markdown.
func ExtractLinks(body []byte, opts Options) ([]Link, error) {
	md := goldmark.New()
	ctx := parser.NewContext()
	root := md.Parser().Parse(text.NewReader(body), parser.WithContext(ctx))

	links := make([]Link, 0)
	_ = gmast.Walk(root, func(n gmast.Node, entering bool) (gmast.WalkStatus, error) {
		if !entering {
			return gmast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *gmast.AutoLink:
			links = append(links, Link{Kind: LinkKindAuto, Destination: string(node.URL(body))})
		case *gmast.Image:
			links = append(links, Link{Kind: LinkKindImage, Destination: string(node.Destination)})
		case *gmast.Link:
			// Goldmark resolves reference-style links to a Link node with a Destination.
			links = append(links, Link{Kind: LinkKindInline, Destination: string(node.Destination)})
		}
		return gmast.WalkContinue, nil
	})

	// Reference definitions are stored in the parse context (not represented as AST nodes).
	refs := ctx.References()
	sort.Slice(refs, func(i, j int) bool {
		return string(refs[i].Label()) < string(refs[j].Label())
	})
	for _, ref := range refs {
		links = append(links, Link{Kind: LinkKindReferenceDefinition, Destination: string(ref.Destination())})
	}

	// Goldmark follows CommonMark strictly. DocBuilder historically relied on
	// permissive destination parsing in some fixer workflows (e.g., destinations
	// containing spaces). Add a best-effort permissive pass to retain
	// minimal-surprise behavior for internal analysis.
	links = append(links, extractPermissiveLinks(body)...)

	return links, nil
}
