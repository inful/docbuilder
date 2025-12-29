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
		// Pattern to match markdown links: [text](path)
		// Negative lookbehind would be ideal but Go doesn't support it,
		// so we use ReplaceAllStringFunc and check for the ! prefix manually
		linkPattern := regexp.MustCompile(`!?\[([^\]]+)\]\(([^)]+)\)`)

		doc.Content = linkPattern.ReplaceAllStringFunc(doc.Content, func(match string) string {
			// Skip image links (those starting with !)
			if strings.HasPrefix(match, "!") {
				return match
			}

			submatches := linkPattern.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}

			text := submatches[1]
			path := submatches[2]

			// Skip absolute URLs, anchors, mailto, etc.
			if strings.HasPrefix(path, "http://") ||
				strings.HasPrefix(path, "https://") ||
				strings.HasPrefix(path, "#") ||
				strings.HasPrefix(path, "mailto:") ||
				strings.HasPrefix(path, "/") {
				return match
			}

			// Rewrite relative link to Hugo-compatible path
			newPath := rewriteLinkPath(path, doc.Repository, doc.Forge, doc.IsIndex, doc.Path)
			return fmt.Sprintf("[%s](%s)", text, newPath)
		})

		return nil, nil
	}
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
func rewriteLinkPath(path, repository, forge string, isIndex bool, docPath string) string {
	// Remove .md extension
	path = strings.TrimSuffix(path, ".md")
	path = strings.TrimSuffix(path, ".markdown")

	// Handle README/index special case - these become section URLs with trailing slash
	if strings.HasSuffix(path, "/README") || strings.HasSuffix(path, "/readme") {
		path = strings.TrimSuffix(path, "/README")
		path = strings.TrimSuffix(path, "/readme")
		path += "/"
	}
	if strings.HasSuffix(path, "/index") {
		path = strings.TrimSuffix(path, "/index")
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
	if strings.HasPrefix(path, "../") {
		// Strip all leading ../ sequences
		for strings.HasPrefix(path, "../") {
			path = strings.TrimPrefix(path, "../")
		}

		// Prepend repository path
		if repository != "" {
			if forge != "" {
				path = fmt.Sprintf("/%s/%s/%s", forge, repository, path)
			} else {
				path = fmt.Sprintf("/%s/%s", repository, path)
			}
		}
		return path + anchor
	}

	// For index files, preserve relative links within the same directory
	if isIndex {
		// Extract directory from document path
		docDir := extractDirectory(docPath)

		// If link doesn't navigate up (..), keep it relative to current section
		if !strings.HasPrefix(path, "/") && docDir != "" {
			// Link is relative to current directory - prepend directory
			if forge != "" {
				path = fmt.Sprintf("/%s/%s/%s", forge, repository, docDir+"/"+path)
			} else {
				path = fmt.Sprintf("/%s/%s", repository, docDir+"/"+path)
			}
			return path + anchor
		}
	}

	// For regular files or absolute paths from index root, prepend repository path
	if !strings.HasPrefix(path, "/") && repository != "" {
		// Extract directory from document path for relative link context
		docDir := extractDirectory(docPath)

		if docDir != "" {
			// Regular file in subdirectory - relative link is relative to that directory
			if forge != "" {
				path = fmt.Sprintf("/%s/%s/%s/%s", forge, repository, docDir, path)
			} else {
				path = fmt.Sprintf("/%s/%s/%s", repository, docDir, path)
			}
		} else {
			// Regular file at repository root
			if forge != "" {
				path = fmt.Sprintf("/%s/%s/%s", forge, repository, path)
			} else {
				path = fmt.Sprintf("/%s/%s", repository, path)
			}
		}
	}

	return path + anchor
}

// extractDirectory extracts the directory part of a Hugo path.
// For "repo/section/file.md" returns "section".
// For "forge/repo/section/file.md" returns "section".
func extractDirectory(hugoPath string) string {
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
		// Build the full path including the document's section
		var fullPath string
		if forge != "" {
			if section != "" {
				fullPath = fmt.Sprintf("/%s/%s/%s/%s", forge, repository, section, path)
			} else {
				fullPath = fmt.Sprintf("/%s/%s/%s", forge, repository, path)
			}
		} else {
			if section != "" {
				fullPath = fmt.Sprintf("/%s/%s/%s", repository, section, path)
			} else {
				fullPath = fmt.Sprintf("/%s/%s", repository, path)
			}
		}
		return fullPath
	}

	return path
}
