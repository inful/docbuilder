package pipeline

import (
	"fmt"
	"regexp"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// rewriteRelativeLinks rewrites relative markdown links to work with Hugo.
func rewriteRelativeLinks(cfg *config.Config) FileTransform {
	return func(doc *Document) ([]*Document, error) {
		// Use an iterative approach instead of regex to avoid catastrophic backtracking
		// This processes the content character-by-character to find valid markdown links
		doc.Content = rewriteLinksIterative(doc.Content, doc.Repository, doc.Forge, doc.IsIndex, doc.Path, doc.IsSingleRepo)
		return nil, nil
	}
}

// rewriteLinksIterative processes markdown content iteratively to avoid regex backtracking.
func rewriteLinksIterative(content, repository, forge string, isIndex bool, docPath string, isSingleRepo bool) string {
	var result strings.Builder
	result.Grow(len(content))

	i := 0
	for i < len(content) {
		// Check if we're at the start of a potential link
		if i < len(content)-1 && content[i] == '[' {
			// Try to process as a link; if successful, advance i and continue
			if newI, processed := tryProcessLink(content, i, repository, forge, isIndex, docPath, isSingleRepo, &result); processed {
				i = newI
				continue
			}
		}

		result.WriteByte(content[i])
		i++
	}

	return result.String()
}

// tryProcessLink attempts to process a markdown link starting at position i.
// Returns the new position and whether a link was successfully processed.
func tryProcessLink(content string, i int, repository, forge string, isIndex bool, docPath string, isSingleRepo bool, result *strings.Builder) (int, bool) {
	// Check if it's an image link (preceded by !)
	isImage := i > 0 && content[i-1] == '!'

	// Find the closing ]
	closeBracket := findClosingBracket(content, i+1)
	if closeBracket == -1 {
		// No closing bracket, not a valid link
		return 0, false
	}

	// Check if there's a ( immediately after ]
	if closeBracket+1 >= len(content) || content[closeBracket+1] != '(' {
		// Not a link, write the content
		result.WriteString(content[i : closeBracket+1])
		return closeBracket + 1, true
	}

	// Find the closing )
	closeParen := findClosingParen(content, closeBracket+2)
	if closeParen == -1 {
		// No closing paren, write what we have
		result.WriteString(content[i : closeBracket+2])
		return closeBracket + 2, true
	}

	// Extract link components
	text := content[i+1 : closeBracket]
	path := content[closeBracket+2 : closeParen]

	// If it's an image or absolute URL, write as-is
	if isImage || isAbsoluteOrSpecialURL(path) {
		result.WriteString(content[i : closeParen+1])
		return closeParen + 1, true
	}

	// Rewrite the relative link
	newPath := rewriteLinkPath(path, repository, forge, isIndex, docPath, isSingleRepo)
	result.WriteByte('[')
	result.WriteString(text)
	result.WriteString("](")
	result.WriteString(newPath)
	result.WriteByte(')')
	return closeParen + 1, true
}

// findClosingBracket finds the next unescaped ] character.
func findClosingBracket(content string, start int) int {
	for i := start; i < len(content); i++ {
		if content[i] == ']' {
			return i
		}
		// Skip newlines to avoid matching across paragraphs
		if content[i] == '\n' {
			// Allow one newline but not multiple (blank line)
			if i+1 < len(content) && content[i+1] == '\n' {
				return -1
			}
		}
	}
	return -1
}

// findClosingParen finds the next unescaped ) character.
func findClosingParen(content string, start int) int {
	for i := start; i < len(content); i++ {
		if content[i] == ')' {
			return i
		}
		// URLs shouldn't span multiple lines
		if content[i] == '\n' {
			return -1
		}
		// Stop at spaces in URLs (markdown links don't have spaces in URLs)
		if content[i] == ' ' {
			return -1
		}
	}
	return -1
}

// isAbsoluteOrSpecialURL checks if a path is absolute or special (http, anchor, mailto, etc).
func isAbsoluteOrSpecialURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "#") ||
		strings.HasPrefix(path, "mailto:") ||
		strings.HasPrefix(path, "/")
}

// rewriteImageLinks rewrites image paths to work with Hugo.
func rewriteImageLinks(doc *Document) ([]*Document, error) {
	// Pattern to match markdown images: ![alt](path)
	imagePattern := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

	doc.Content = imagePattern.ReplaceAllStringFunc(doc.Content, func(match string) string {
		submatches := imagePattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		alt := submatches[1]
		path := submatches[2]

		// Skip absolute URLs
		if strings.HasPrefix(path, "http://") ||
			strings.HasPrefix(path, "https://") ||
			strings.HasPrefix(path, "/") {
			return match
		}

		// Rewrite relative image path accounting for document's section
		newPath := rewriteImagePath(path, doc.Repository, doc.Forge, doc.Section)
		return fmt.Sprintf("![%s](%s)", alt, newPath)
	})

	// Also handle HTML img tags: <img src="path" ...>
	htmlImgPattern := regexp.MustCompile(`<img\s+([^>]*\s+)?src="([^"]+)"([^>]*)>`)

	doc.Content = htmlImgPattern.ReplaceAllStringFunc(doc.Content, func(match string) string {
		submatches := htmlImgPattern.FindStringSubmatch(match)
		if len(submatches) < 4 {
			return match
		}

		beforeSrc := submatches[1]
		path := submatches[2]
		afterSrc := submatches[3]

		// Skip absolute URLs
		if strings.HasPrefix(path, "http://") ||
			strings.HasPrefix(path, "https://") ||
			strings.HasPrefix(path, "/") {
			return match
		}

		// Rewrite relative image path
		newPath := rewriteImagePath(path, doc.Repository, doc.Forge, doc.Section)
		return fmt.Sprintf("<img %ssrc=\"%s\"%s>", beforeSrc, newPath, afterSrc)
	})

	return nil, nil
}

// rewriteLinkPath rewrites a link path based on the document's context.
func rewriteLinkPath(path, repository, forge string, isIndex bool, docPath string, isSingleRepo bool) string {
	// Strip leading ./ from relative paths (e.g., ./api-guide.md -> api-guide.md)
	path = strings.TrimPrefix(path, "./")

	// Remove .md extension
	path = strings.TrimSuffix(path, ".md")
	path = strings.TrimSuffix(path, ".markdown")

	// Handle README/index special case - these become section URLs with trailing slash
	if strings.HasSuffix(path, "/README") || strings.HasSuffix(path, "/readme") {
		path = strings.TrimSuffix(path, "/README")
		path = strings.TrimSuffix(path, "/readme")
		path += "/"
	}
	if before, ok := strings.CutSuffix(path, "/index"); ok {
		path = before
		path += "/"
	}

	// Handle anchor links
	anchorIdx := strings.Index(path, "#")
	var anchor string
	if anchorIdx != -1 {
		anchor = path[anchorIdx:]
		path = path[:anchorIdx]
	}

	// Skip empty paths (pure anchors)
	if path == "" {
		return anchor
	}

	// Handle relative paths that navigate up directories (../)
	if after, ok := strings.CutPrefix(path, "../"); ok {
		// Strip all leading ../ sequences
		path = after
		for strings.HasPrefix(path, "../") {
			path = strings.TrimPrefix(path, "../")
		}

		// Prepend repository path (skip if single-repo build)
		if !isSingleRepo && repository != "" {
			if forge != "" {
				path = fmt.Sprintf("/%s/%s/%s", forge, repository, path)
			} else {
				path = fmt.Sprintf("/%s/%s", repository, path)
			}
		} else {
			// Single-repo mode: ensure leading slash
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
		}
		return path + anchor
	}

	// For index files, preserve relative links within the same directory
	if isIndex {
		// Extract directory from document path
		docDir := extractDirectory(docPath, isSingleRepo)

		// If link doesn't navigate up (..), keep it relative to current section
		if !strings.HasPrefix(path, "/") && docDir != "" {
			// Link is relative to current directory - prepend directory
			if !isSingleRepo {
				if forge != "" {
					path = fmt.Sprintf("/%s/%s/%s", forge, repository, docDir+"/"+path)
				} else {
					path = fmt.Sprintf("/%s/%s", repository, docDir+"/"+path)
				}
			} else {
				// Single-repo mode: skip repository namespace
				path = fmt.Sprintf("/%s/%s", docDir, path)
			}
			return path + anchor
		}
	}

	// For regular files or absolute paths from index root, prepend repository path
	if !strings.HasPrefix(path, "/") && repository != "" {
		// Extract directory from document path for relative link context
		docDir := extractDirectory(docPath, isSingleRepo)
		if !isSingleRepo {
			path = buildFullPath(forge, repository, docDir, path)
		} else {
			// Single-repo mode: skip repository namespace
			if docDir != "" {
				path = "/" + docDir + "/" + path
			} else {
				path = "/" + path
			}
		}
	}

	return path + anchor
}

// extractDirectory extracts the directory part of a Hugo path.
// For "repo/section/file.md" returns "section".
// For "forge/repo/section/file.md" returns "section".
// extractDirectory extracts the section/directory path from a Hugo document path.
// For single-repo builds, all segments except the filename are returned.
// For multi-repo builds, the repository (and optional forge) segments are stripped first.
func extractDirectory(hugoPath string, isSingleRepo bool) string {
	// Remove leading slash if present
	hugoPath = strings.TrimPrefix(hugoPath, "/")

	// Strip content/ prefix if present (Hugo content directory)
	hugoPath = strings.TrimPrefix(hugoPath, "content/")

	// Split by /
	segments := strings.Split(hugoPath, "/")
	if len(segments) <= 1 {
		// Just filename or empty
		return ""
	}

	// Remove filename (last segment)
	segments = segments[:len(segments)-1]

	// For single-repo builds, return all remaining segments (no repo/forge to strip)
	if isSingleRepo {
		return strings.Join(segments, "/")
	}

	// Detect forge namespace pattern
	// Common forge names: gitlab, github, forgejo, gitea (all <= 8 chars)
	hasForge := len(segments) >= 3 && len(segments[0]) <= 8

	if hasForge {
		// forge/repo/section... format
		// Return everything after repo (index 1)
		if len(segments) > 2 {
			return strings.Join(segments[2:], "/")
		}
		return ""
	}

	// repo/section... format
	// Return everything after repo (index 0)
	if len(segments) > 1 {
		return strings.Join(segments[1:], "/")
	}
	return ""
}

// rewriteImagePath rewrites an image path based on the document's context.
// The path is relative to the document's location (section).
func rewriteImagePath(path, repository, forge, section string) string {
	// Normalize the path first (remove ./ prefix, collapse ../, lowercase)
	path = strings.TrimPrefix(path, "./")

	// Lowercase the entire path including filename and extension for URL compatibility
	path = strings.ToLower(path)

	// Prepend repository and section path if relative
	if !strings.HasPrefix(path, "/") && repository != "" {
		return buildFullPath(forge, repository, section, path)
	}

	return path
}

// buildFullPath constructs a full path with forge, repository, and section components.
func buildFullPath(forge, repository, section, path string) string {
	parts := make([]string, 0, 5)
	parts = append(parts, "")

	if forge != "" {
		parts = append(parts, forge)
	}

	parts = append(parts, repository)

	if section != "" {
		parts = append(parts, section)
	}

	parts = append(parts, path)

	return strings.Join(parts, "/")
}
