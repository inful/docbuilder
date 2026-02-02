package templates

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// TemplateLink represents a template discovered from the rendered site.
type TemplateLink struct {
	Type string
	URL  string
}

// ParseTemplateDiscovery extracts template links from a rendered templates taxonomy page.
func ParseTemplateDiscovery(baseURL string, r io.Reader) ([]TemplateLink, error) {
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
				templateType := deriveTemplateType(extractText(n), href)
				if templateType != "" {
					results = append(results, TemplateLink{
						Type: templateType,
						URL:  resolveURL(parsedBase, href),
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

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

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
