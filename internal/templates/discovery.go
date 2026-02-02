package templates

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// TemplateLink represents a template discovered from the rendered documentation site.
//
// Templates are discovered by parsing the HTML from the /categories/templates/ taxonomy
// page, which contains links to individual template pages.
type TemplateLink struct {
	// Type is the template identifier (e.g., "adr", "guide") extracted from the link.
	Type string

	// URL is the fully resolved URL to the template page.
	URL string

	// Name is a human-friendly display name, derived from the anchor text or type.
	Name string
}

// ParseTemplateDiscovery parses the HTML content of a template discovery page
// (typically /categories/templates/) and extracts links to individual templates.
//
// The function looks for anchor tags (<a>) with href attributes containing ".template/"
// and extracts the template type from either the anchor text or the URL path.
//
// Parameters:
//   - r: HTML content reader (typically from HTTP response body)
//   - baseURL: Base URL of the documentation site for resolving relative links
//
// Returns:
//   - A slice of TemplateLink structs, one per discovered template
//   - An error if parsing fails or no templates are found
//
// Example:
//
//	links, err := ParseTemplateDiscovery("https://docs.example.com", htmlReader)
//	if err != nil {
//	    return err
//	}
//	for _, link := range links {
//	    fmt.Printf("Found template: %s at %s\n", link.Type, link.URL)
//	}
func ParseTemplateDiscovery(r io.Reader, baseURL string) ([]TemplateLink, error) {
	if baseURL == "" {
		return nil, errors.New("base URL is required")
	}
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}

	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse discovery HTML: %w", err)
	}

	var results []TemplateLink
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := getAttr(n, "href")
			if strings.Contains(href, ".template/") {
				anchorText := extractText(n)
				templateType := deriveTemplateType(anchorText, href)
				if templateType != "" {
					name := anchorText
					if name == "" {
						name = templateType // Fallback to type if no anchor text
					}
					results = append(results, TemplateLink{
						Type: templateType,
						URL:  resolveURL(parsedBase, href),
						Name: name,
					})
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(results) == 0 {
		return nil, errors.New("no template links discovered")
	}

	return results, nil
}

// deriveTemplateType extracts the template type identifier from anchor text or URL.
//
// It first tries to extract from the anchor text (removing ".template" suffix),
// then falls back to parsing the URL path segments.
func deriveTemplateType(anchorText, href string) string {
	text := strings.TrimSpace(anchorText)
	if text != "" {
		return strings.TrimSuffix(text, ".template")
	}

	u, err := url.Parse(href)
	if err != nil {
		return ""
	}

	path := strings.TrimSuffix(u.Path, "/")
	if path == "" {
		return ""
	}

	segments := strings.Split(path, "/")
	for i := len(segments) - 1; i >= 0; i-- {
		if strings.Contains(segments[i], ".template") {
			return strings.TrimSuffix(segments[i], ".template")
		}
	}

	return strings.TrimSuffix(segments[len(segments)-1], ".template")
}

// resolveURL resolves a relative URL against a base URL.
//
// If the href is already absolute or parsing fails, it returns the href unchanged.
func resolveURL(base *url.URL, href string) string {
	if base == nil {
		return href
	}
	rel, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(rel).String()
}

// getAttr extracts an attribute value from an HTML node.
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// extractText recursively extracts all text content from an HTML node and its children.
func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data)
	}

	var text strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text.WriteString(extractText(c))
	}

	return strings.TrimSpace(text.String())
}
