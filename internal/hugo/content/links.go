package content

import (
	"fmt"
	"regexp"
	"strings"
)

// RewriteRelativeMarkdownLinks rewrites relative markdown links that end with .md/.markdown
// to their extensionless form with trailing slashes for Hugo's pretty URLs. Anchors are preserved.
// Rules:
//   - Only adjust links that are NOT absolute (http/https/mailto) or anchor-only ('#')
//   - Rewrites foo.md -> foo/, foo.md#anchor -> foo/#anchor
//   - Supports ./ and ../ prefixes transparently (kept as-is except extension removal)
//   - If repositoryName is provided, links starting with / are treated as repository-root-relative
//     and are prefixed with /{forge}/{repository}/ or /{repository}/ depending on forge presence
//   - isIndexPage: if true, the file is a _index.md at the section root, so relative links don't need extra ../
//   - Leaves README.md alone if removal would produce empty target (defensive)
func RewriteRelativeMarkdownLinks(content string, repositoryName string, forgeName string, isIndexPage bool) string {
	linkRe := regexp.MustCompile(`\[(?P<text>[^\]]+)\]\((?P<link>[^)]+)\)`)
	return linkRe.ReplaceAllStringFunc(content, func(m string) string {
		matches := linkRe.FindStringSubmatch(m)
		if len(matches) != 3 {
			return m
		}
		text := matches[1]
		link := matches[2]
		low := strings.ToLower(link)
		// Skip external links, anchors, and mailto
		if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "mailto:") || strings.HasPrefix(link, "#") {
			return m
		}

		// Handle repository-root-relative links (start with /)
		// These need to be prefixed with repository (and forge if present) to work in Hugo
		if strings.HasPrefix(link, "/") && repositoryName != "" {
			anchor := ""
			if idx := strings.IndexByte(link, '#'); idx >= 0 {
				anchor = link[idx:]
				link = link[:idx]
			}
			lowerPath := strings.ToLower(link)
			trimmed := link
			if strings.HasSuffix(lowerPath, ".md") {
				trimmed = link[:len(link)-3]
			} else if strings.HasSuffix(lowerPath, ".markdown") {
				trimmed = link[:len(link)-9]
			}
			if trimmed == "" {
				return m
			}
			// Add trailing slash for Hugo's pretty URLs
			if !strings.HasSuffix(trimmed, "/") {
				trimmed += "/"
			}
			// Build the Hugo-absolute path with repository (and forge if present)
			var prefix string
			if forgeName != "" {
				prefix = fmt.Sprintf("/%s/%s", forgeName, repositoryName)
			} else {
				prefix = fmt.Sprintf("/%s", repositoryName)
			}
			return fmt.Sprintf("[%s](%s%s%s)", text, prefix, trimmed, anchor)
		}

		// Handle page-relative links (no leading /)
		// Hugo URLs are one level deeper than source files (page name becomes a directory)
		// So ../foo.md in source needs to become ../../foo/ in Hugo
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
			// Normalize ./ prefix for regular pages (non-index)
			// For index pages (_index.md), preserve ./ to maintain section-relative context
			// For regular pages, ./foo.md and foo.md are equivalent and both need ../
			hasCurrentDirPrefix := strings.HasPrefix(trimmed, "./")
			if hasCurrentDirPrefix && !isIndexPage {
				trimmed = trimmed[2:] // Remove ./ for regular pages
			}
			// Add trailing slash for Hugo's pretty URLs
			if !strings.HasSuffix(trimmed, "/") {
				trimmed += "/"
			}
			// Adjust relative paths for Hugo's deeper URL structure
			// Hugo URLs are one level deeper than source files because the page name becomes a directory
			// For regular pages (not _index.md):
			//   - Same-directory link:   architecture.md       → ../architecture/
			//   - Same-directory explicit: ./ref.md            → ../ref/  (after ./ removal)
			//   - Parent directory link: ../other.md           → ../../other/
			//   - Child directory link:  guide/setup.md        → ../guide/setup/
			// For index pages (_index.md): relative links stay as-is (already at section root)
			//   - Same-directory link:   guide.md              → guide/
			//   - Same-directory explicit: ./guide.md          → ./guide/ (preserve ./)
			//   - Parent directory link: ../other.md           → ../other/
			if !isIndexPage {
				trimmed = "../" + trimmed
			}
			return fmt.Sprintf("[%s](%s%s)", text, trimmed, anchor)
		}
		return m
	})
}
