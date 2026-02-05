package templates

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// TemplateMeta contains metadata extracted from docbuilder:* HTML meta tags.
//
// All metadata is stored as strings (JSON for complex types) and must be parsed
// separately using ParseTemplateSchema, ParseTemplateDefaults, etc.
type TemplateMeta struct {
	// Type is the canonical template identifier (e.g., "adr", "guide").
	// Required. Extracted from "docbuilder:template.type" meta tag.
	Type string

	// Name is the human-friendly display name shown in template lists.
	// Required. Extracted from "docbuilder:template.name" meta tag.
	Name string

	// OutputPath is a Go template string defining where generated files are written.
	// Required. Extracted from "docbuilder:template.output_path" meta tag.
	// Example: "adr/adr-{{ printf \"%03d\" (nextInSequence \"adr\") }}-{{ .Slug }}.md"
	OutputPath string

	// Description is an optional brief description of the template.
	// Extracted from "docbuilder:template.description" meta tag.
	Description string

	// Schema is a JSON string defining input fields and their types.
	// Extracted from "docbuilder:template.schema" meta tag.
	// See TemplateSchema for the structure.
	Schema string

	// Defaults is a JSON object string providing default values for fields.
	// Extracted from "docbuilder:template.defaults" meta tag.
	Defaults string

	// Sequence is a JSON object string defining sequential numbering configuration.
	// Extracted from "docbuilder:template.sequence" meta tag.
	// See SequenceDefinition for the structure.
	Sequence string
}

// TemplatePage represents a fully parsed template page with metadata and body.
type TemplatePage struct {
	// Meta contains all template metadata extracted from HTML meta tags.
	Meta TemplateMeta

	// Body is the raw markdown template content extracted from the code block.
	// This is the template that will be rendered with user inputs.
	Body string
}

// ParseTemplatePage parses an HTML template page and extracts metadata and markdown body.
//
// The function:
//   - Extracts metadata from <meta property="docbuilder:template.*"> tags in the <head>
//   - Finds the first <pre><code class="language-markdown"> block in the <body>
//   - Validates that required metadata (type, name, output_path) is present
//   - Ensures exactly one markdown code block exists
//
// Parameters:
//   - r: HTML content reader (typically from HTTP response body)
//
// Returns:
//   - A TemplatePage with parsed metadata and body
//   - An error if required metadata is missing, no code block is found, or multiple blocks exist
//
// Example:
//
//	page, err := ParseTemplatePage(htmlReader)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Template: %s (%s)\n", page.Meta.Name, page.Meta.Type)
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

	missing := missingRequiredTemplateMeta(result.Meta)
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required template metadata: %s", strings.Join(missing, ", "))
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

func missingRequiredTemplateMeta(meta TemplateMeta) []string {
	var missing []string
	if strings.TrimSpace(meta.Type) == "" {
		missing = append(missing, "docbuilder:template.type")
	}
	if strings.TrimSpace(meta.Name) == "" {
		missing = append(missing, "docbuilder:template.name")
	}
	if strings.TrimSpace(meta.OutputPath) == "" {
		missing = append(missing, "docbuilder:template.output_path")
	}
	return missing
}

// isMarkdownCodeNode checks if an HTML node is a markdown code block.
//
// A markdown code block is a <code> element inside a <pre> element with a class
// containing "language-markdown", "language-md", "lang-markdown", "lang-md", or "markdown".
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
