package linkverify

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// Link represents an extracted link from HTML content.
type Link struct {
	URL        string // The URL or path
	Text       string // Link text/title
	Tag        string // HTML tag (a, img, script, link, etc.)
	Attribute  string // Attribute containing the link (href, src, etc.)
	IsInternal bool   // True if link is internal to the site
	Line       int    // Approximate line number in HTML
}

// ExtractLinks extracts all links from an HTML file.
func ExtractLinks(htmlPath string, baseURL string) ([]*Link, error) {
	file, err := os.Open(filepath.Clean(htmlPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open HTML file: %w", err)
	}
	defer func() {
		_ = file.Close() // Ignore close errors on read-only operation
	}()

	return ExtractLinksFromReader(file, baseURL)
}

// ExtractLinksFromReader extracts all links from an HTML reader.
func ExtractLinksFromReader(r io.Reader, baseURL string) ([]*Link, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	var links []*Link
	var lineNum int

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			lineNum++

			// Extract links based on element type
			switch n.Data {
			case "a":
				if href := getAttr(n, "href"); href != "" {
					links = append(links, &Link{
						URL:        href,
						Text:       extractText(n),
						Tag:        "a",
						Attribute:  "href",
						IsInternal: isInternalLink(href, base),
						Line:       lineNum,
					})
				}
			case "img":
				if src := getAttr(n, "src"); src != "" {
					links = append(links, &Link{
						URL:        src,
						Text:       getAttr(n, "alt"),
						Tag:        "img",
						Attribute:  "src",
						IsInternal: isInternalLink(src, base),
						Line:       lineNum,
					})
				}
			case "script":
				if src := getAttr(n, "src"); src != "" {
					links = append(links, &Link{
						URL:        src,
						Text:       "",
						Tag:        "script",
						Attribute:  "src",
						IsInternal: isInternalLink(src, base),
						Line:       lineNum,
					})
				}
			case "link":
				if href := getAttr(n, "href"); href != "" {
					links = append(links, &Link{
						URL:        href,
						Text:       getAttr(n, "rel"),
						Tag:        "link",
						Attribute:  "href",
						IsInternal: isInternalLink(href, base),
						Line:       lineNum,
					})
				}
			case "iframe":
				if src := getAttr(n, "src"); src != "" {
					links = append(links, &Link{
						URL:        src,
						Text:       getAttr(n, "title"),
						Tag:        "iframe",
						Attribute:  "src",
						IsInternal: isInternalLink(src, base),
						Line:       lineNum,
					})
				}
			case "video", "audio":
				if src := getAttr(n, "src"); src != "" {
					links = append(links, &Link{
						URL:        src,
						Text:       "",
						Tag:        n.Data,
						Attribute:  "src",
						IsInternal: isInternalLink(src, base),
						Line:       lineNum,
					})
				}
			case "source":
				if src := getAttr(n, "src"); src != "" {
					links = append(links, &Link{
						URL:        src,
						Text:       "",
						Tag:        "source",
						Attribute:  "src",
						IsInternal: isInternalLink(src, base),
						Line:       lineNum,
					})
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)

	return links, nil
}

// getAttr retrieves an attribute value from an HTML node.
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// extractText extracts text content from an HTML node and its children.
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

// isInternalLink determines if a URL is internal to the site.
func isInternalLink(linkURL string, baseURL *url.URL) bool {
	// Skip special protocols
	if strings.HasPrefix(linkURL, "mailto:") ||
		strings.HasPrefix(linkURL, "tel:") ||
		strings.HasPrefix(linkURL, "javascript:") ||
		strings.HasPrefix(linkURL, "#") {
		return true // These are not external links
	}

	// Parse the link
	u, err := url.Parse(linkURL)
	if err != nil {
		return false
	}

	// Relative URLs are internal
	if u.Scheme == "" || u.Host == "" {
		return true
	}

	// Compare hosts
	if baseURL != nil && u.Host == baseURL.Host {
		return true
	}

	return false
}

// FilterLinks filters links based on criteria.
func FilterLinks(links []*Link, includeInternal, includeExternal bool) []*Link {
	var filtered []*Link
	for _, link := range links {
		if link.IsInternal && includeInternal {
			filtered = append(filtered, link)
		} else if !link.IsInternal && includeExternal {
			filtered = append(filtered, link)
		}
	}
	return filtered
}

// ShouldVerifyLink determines if a link should be verified based on global rules.
// Note: Configuration-based skipping (skipEditLinks) is handled separately in verifyPage.
func ShouldVerifyLink(link *Link) bool {
	// Skip anchors
	if strings.HasPrefix(link.URL, "#") {
		return false
	}

	// Skip special protocols
	if strings.HasPrefix(link.URL, "mailto:") ||
		strings.HasPrefix(link.URL, "tel:") ||
		strings.HasPrefix(link.URL, "javascript:") ||
		strings.HasPrefix(link.URL, "data:") {
		return false
	}

	// Skip empty links
	if link.URL == "" {
		return false
	}

	// Skip Hugo-generated files that are optional features
	if isOptionalHugoFeature(link.URL) {
		return false
	}

	return true
}

// isEditLink checks if a URL is an edit link that requires authentication.
// Edit links are generated for "Edit this page" functionality and point to
// forge-specific edit interfaces (GitHub, GitLab, Forgejo).
func isEditLink(linkURL string) bool {
	// Parse URL to check path components
	u, err := url.Parse(linkURL)
	if err != nil {
		return false
	}

	path := u.Path

	// GitHub edit links: /owner/repo/edit/branch/path
	// GitLab edit links: /owner/repo/-/edit/branch/path
	// Forgejo edit links: /owner/repo/_edit/branch/path
	if strings.Contains(path, "/edit/") ||
		strings.Contains(path, "/-/edit/") ||
		strings.Contains(path, "/_edit/") {
		return true
	}

	return false
}

// isOptionalHugoFeature checks if a URL points to an optional Hugo-generated file.
// These files are only generated when specific features are enabled.
func isOptionalHugoFeature(linkURL string) bool {
	// Parse URL to get clean path
	u, err := url.Parse(linkURL)
	if err != nil {
		return false
	}

	path := u.Path

	// RSS/Atom feeds (.xml files) - only generated if RSS is enabled
	if strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, "/index.xml") {
		return true
	}

	// Search index (.json files) - only generated if search is enabled
	if strings.HasSuffix(path, ".json") || strings.HasSuffix(path, "/index.json") {
		return true
	}

	// Sitemap - only if sitemap generation is enabled
	if strings.Contains(path, "sitemap") {
		return true
	}

	// robots.txt - optional
	if strings.HasSuffix(path, "robots.txt") {
		return true
	}

	return false
}
