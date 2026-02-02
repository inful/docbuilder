package templates

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// TemplateMeta contains metadata extracted from docbuilder:* meta tags.
type TemplateMeta struct {
	Type        string
	Name        string
	OutputPath  string
	Description string
	Schema      string
	Defaults    string
	Sequence    string
}

// TemplatePage represents a parsed template page and its markdown body.
type TemplatePage struct {
	Meta TemplateMeta
	Body string
}

// ParseTemplatePage extracts template metadata and the markdown body from a template page.
func ParseTemplatePage(r io.Reader) (*TemplatePage, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse template HTML: %w", err)
	}

	meta := make(map[string]string)
	var markdownBlocks []string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				if prop := getAttr(n, "property"); prop != "" {
					meta[prop] = getAttr(n, "content")
				}
			case "code":
				if isMarkdownCodeNode(n) {
					markdownBlocks = append(markdownBlocks, strings.TrimSpace(extractText(n)))
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	result := &TemplatePage{
		Meta: TemplateMeta{
			Type:        meta["docbuilder:template.type"],
			Name:        meta["docbuilder:template.name"],
			OutputPath:  meta["docbuilder:template.output_path"],
			Description: meta["docbuilder:template.description"],
			Schema:      meta["docbuilder:template.schema"],
			Defaults:    meta["docbuilder:template.defaults"],
			Sequence:    meta["docbuilder:template.sequence"],
		},
	}

	if result.Meta.Type == "" || result.Meta.Name == "" || result.Meta.OutputPath == "" {
		return nil, errors.New("missing required template metadata")
	}

	if len(markdownBlocks) == 0 {
		return nil, errors.New("template page missing markdown code block")
	}
	if len(markdownBlocks) > 1 {
		return nil, errors.New("template page contains multiple markdown code blocks")
	}

	result.Body = markdownBlocks[0]
	return result, nil
}

func isMarkdownCodeNode(n *html.Node) bool {
	if n == nil || n.Data != "code" {
		return false
	}
	if n.Parent == nil || n.Parent.Data != "pre" {
		return false
	}

	class := strings.ToLower(getAttr(n, "class"))
	return strings.Contains(class, "language-markdown") ||
		strings.Contains(class, "language-md") ||
		strings.Contains(class, "lang-markdown") ||
		strings.Contains(class, "lang-md") ||
		strings.Contains(class, "markdown")
}
