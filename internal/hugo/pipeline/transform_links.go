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
		strings.HasPrefix(path, "mailto:")
}

// rewriteImageLinks rewrites image paths to work with Hugo.
func rewriteImageLinks(doc *Document) ([]*Document, error) {
	// Pattern to match markdown images: ![alt](path)
	imagePattern := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

	lowerPathPreserveQueryAndFragment := func(p string) string {
		cut := len(p)
		if i := strings.IndexByte(p, '?'); i >= 0 && i < cut {
			cut = i
		}
		if i := strings.IndexByte(p, '#'); i >= 0 && i < cut {
			cut = i
		}
		return strings.ToLower(p[:cut]) + p[cut:]
	}

	doc.Content = imagePattern.ReplaceAllStringFunc(doc.Content, func(match string) string {
		submatches := imagePattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		alt := submatches[1]
		path := submatches[2]

		// Skip absolute URLs
		if strings.HasPrefix(path, "http://") ||
			strings.HasPrefix(path, "https://") {
			return match
		}

		// Normalize root-relative paths to lowercase (DocBuilder writes content paths lowercased)
		if strings.HasPrefix(path, "/") {
			newPath := lowerPathPreserveQueryAndFragment(path)
			return fmt.Sprintf("![%s](%s)", alt, newPath)
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
			strings.HasPrefix(path, "https://") {
			return match
		}

		// Normalize root-relative paths to lowercase
		if strings.HasPrefix(path, "/") {
			newPath := lowerPathPreserveQueryAndFragment(path)
			return fmt.Sprintf("<img %ssrc=\"%s\"%s>", beforeSrc, newPath, afterSrc)
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

	// Preserve query + anchor while rewriting the path itself.
	anchor := ""
	if idx := strings.IndexByte(path, '#'); idx >= 0 {
		anchor = path[idx:]
		path = path[:idx]
	}
	query := ""
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		query = path[idx:]
		path = path[:idx]
	}
	suffix := query + anchor

	// Remove .md/.markdown extension (case-insensitive)
	lowerPath := strings.ToLower(path)
	if strings.HasSuffix(lowerPath, ".md") {
		path = path[:len(path)-3]
		lowerPath = lowerPath[:len(lowerPath)-3]
	} else if strings.HasSuffix(lowerPath, ".markdown") {
		path = path[:len(path)-9]
		lowerPath = lowerPath[:len(lowerPath)-9]
	}

	// Handle README/index special case - these become section URLs with trailing slash
	if strings.HasSuffix(lowerPath, "/readme") {
		path = path[:len(path)-len("/README")]
		lowerPath = lowerPath[:len(lowerPath)-len("/readme")]
		path += "/"
		lowerPath += "/"
	}
	if before, ok := strings.CutSuffix(lowerPath, "/index"); ok {
		path = path[:len(before)] + "/"
	}

	// Handle repository-root-relative links (start with /): normalize case and namespace.
	if rel, ok := strings.CutPrefix(path, "/"); ok {
		rel = strings.ToLower(rel)

		// Multi-repo builds need repository (and optional forge) prefix.
		if !isSingleRepo && repository != "" {
			return buildFullPath(forge, repository, "", rel) + suffix
		}

		// Single-repo mode: keep site-root path.
		return "/" + rel + suffix
	}

	// Skip empty paths (pure anchors)
	if path == "" {
		return suffix
	}

	// Handle relative paths that navigate up directories (../)
	if after, ok := strings.CutPrefix(path, "../"); ok {
		path = handleParentDirNavigation(after, repository, forge, isSingleRepo)
		return path + suffix
	}

	// For index files, preserve relative links within the same directory
	if isIndex {
		if result, handled := handleIndexFileLink(path, docPath, repository, forge, isSingleRepo, suffix); handled {
			return result
		}
	}

	// For regular files or absolute paths from index root, prepend repository path
	if !strings.HasPrefix(path, "/") && repository != "" {
		path = handleRegularFileLink(path, docPath, repository, forge, isSingleRepo)
	}

	return path + suffix
}

// handleParentDirNavigation handles links with ../ navigation.
func handleParentDirNavigation(path, repository, forge string, isSingleRepo bool) string {
	// Strip all leading ../ sequences
	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}

	// Prepend repository path (skip if single-repo build)
	if !isSingleRepo && repository != "" {
		if forge != "" {
			return fmt.Sprintf("/%s/%s/%s", forge, repository, path)
		}
		return fmt.Sprintf("/%s/%s", repository, path)
	}

	// Single-repo mode: ensure leading slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// handleIndexFileLink handles relative links from index files.
func handleIndexFileLink(path, docPath, repository, forge string, isSingleRepo bool, anchor string) (string, bool) {
	// Extract directory from document path
	docDir := extractDirectory(docPath, isSingleRepo, forge)

	// If link doesn't navigate up (..), keep it relative to current section
	if strings.HasPrefix(path, "/") || docDir == "" {
		return "", false // Not handled, continue with regular processing
	}

	// Link is relative to current directory - prepend directory
	if !isSingleRepo {
		if forge != "" {
			return fmt.Sprintf("/%s/%s/%s%s", forge, repository, docDir+"/"+path, anchor), true
		}
		return fmt.Sprintf("/%s/%s%s", repository, docDir+"/"+path, anchor), true
	}

	// Single-repo mode: skip repository namespace
	return fmt.Sprintf("/%s/%s%s", docDir, path, anchor), true
}

// handleRegularFileLink handles relative links from regular (non-index) files.
func handleRegularFileLink(path, docPath, repository, forge string, isSingleRepo bool) string {
	// Extract directory from document path for relative link context
	docDir := extractDirectory(docPath, isSingleRepo, forge)

	if !isSingleRepo {
		return buildFullPath(forge, repository, docDir, path)
	}

	// Single-repo mode: skip repository namespace
	if docDir != "" {
		return "/" + docDir + "/" + path
	}
	return "/" + path
}

// extractDirectory extracts the directory part of a Hugo path.
// For "repo/section/file.md" returns "section".
// For "forge/repo/section/file.md" returns "section".
// extractDirectory extracts the section/directory path from a Hugo document path.
// For single-repo builds, all segments except the filename are returned.
// For multi-repo builds, the repository (and optional forge) segments are stripped first.
func extractDirectory(hugoPath string, isSingleRepo bool, forge string) string {
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

	// Check if forge namespace is present
	if forge != "" {
		// forge/repo/section... format
		// Return everything after repo (index 1)
		if len(segments) > 2 {
			return strings.Join(segments[2:], "/")
		}
		return ""
	}

	// repo/section... format (no forge)
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
		parts = append(parts, strings.ToLower(forge))
	}

	parts = append(parts, strings.ToLower(repository))

	if section != "" {
		parts = append(parts, strings.ToLower(section))
	}

	parts = append(parts, strings.ToLower(path))

	return strings.Join(parts, "/")
}
