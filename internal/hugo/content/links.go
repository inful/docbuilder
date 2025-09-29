package content

import (
	"fmt"
	"regexp"
	"strings"
)

// RewriteRelativeMarkdownLinks rewrites relative markdown links that end with .md/.markdown
// to their extensionless form so Hugo's pretty URLs work. Anchors are preserved.
// Rules:
// - Only adjust links that are NOT absolute (http/https/mailto), not starting with '#', and not starting with '/'
// - Rewrites foo.md -> foo, foo.md#anchor -> foo#anchor
// - Supports ./ and ../ prefixes transparently (kept as-is except extension removal)
// - Leaves README.md alone if removal would produce empty target (defensive)
func RewriteRelativeMarkdownLinks(content string) string {
	linkRe := regexp.MustCompile(`\[(?P<text>[^\]]+)\]\((?P<link>[^)]+)\)`)
	return linkRe.ReplaceAllStringFunc(content, func(m string) string {
		matches := linkRe.FindStringSubmatch(m)
		if len(matches) != 3 {
			return m
		}
		text := matches[1]
		link := matches[2]
		low := strings.ToLower(link)
		if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "mailto:") || strings.HasPrefix(link, "#") || strings.HasPrefix(link, "/") {
			return m
		}
		anchor := ""
		if idx := strings.IndexByte(link, '#'); idx >= 0 {
			anchor = link[idx:]
			link = link[:idx]
		}
		lowerPath := strings.ToLower(link)
		if strings.HasSuffix(lowerPath, ".md") || strings.HasSuffix(lowerPath, ".markdown") {
			trimmed := link
			if strings.HasSuffix(lowerPath, ".md") {
				trimmed = link[:len(link)-3]
			} else if strings.HasSuffix(lowerPath, ".markdown") {
				trimmed = link[:len(link)-9]
			}
			if trimmed == "" {
				return m
			}
			return fmt.Sprintf("[%s](%s%s)", text, trimmed, anchor)
		}
		return m
	})
}
