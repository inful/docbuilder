package pipeline

import (
	"regexp"
	"strings"
)

// extractIndexTitle extracts H1 heading from index files to use as title.
// Only processes index files and only if H1 is at the start of content.
// For repository-level indexes, always uses repository name for consistency.
func extractIndexTitle(doc *Document) ([]*Document, error) {
	if !doc.IsIndex {
		return nil, nil // Only process index files
	}

	// Skip if title already exists and is not a fallback
	// Allow extraction if title is "Untitled", "_index", or the doc name (fallback values)
	if existingTitle, hasTitle := doc.FrontMatter["title"].(string); hasTitle {
		if existingTitle != untitledDocTitle && existingTitle != indexFileSuffix && existingTitle != doc.Name {
			return nil, nil // Skip - real title already exists
		}
	}

	// PRIORITY 1: Repository-level index files ALWAYS use repository name for consistency
	// This includes:
	// 1. Repository root indexes where Section is empty (index.md at docs/ root)
	// 2. Docs base sections where DocsBase == Section (nested docs paths like docs/documentation)
	isRepositoryRoot := doc.Section == "" && doc.Repository != "" && doc.IsIndex
	isDocsBase := doc.Section != "" && doc.DocsBase == doc.Section && doc.Repository != ""

	if isRepositoryRoot || isDocsBase {
		repoTitle := titleCase(doc.Repository)
		if repoTitle != "" {
			doc.FrontMatter["title"] = repoTitle
		}
		return nil, nil
	}

	// PRIORITY 2: Extract H1 title for section-level index files (subsections within a repo)
	return extractH1AsTitle(doc)
}

// extractH1AsTitle extracts the first H1 heading to use as the document title.
// Only extracts if:
// - No proper title exists in frontmatter (or it's a fallback like filename)
// - H1 is at the start of content (no text before it)
// This allows users to write title once in H1, which gets promoted to frontmatter.
func extractH1AsTitle(doc *Document) ([]*Document, error) {
	// Skip if title already exists and is not a fallback
	// Fallback titles to replace: "Untitled", "_index", or the doc name (auto-generated from filename)
	if existingTitle, hasTitle := doc.FrontMatter["title"].(string); hasTitle {
		// Keep existing non-fallback titles
		if existingTitle != untitledDocTitle && existingTitle != indexFileSuffix && existingTitle != doc.Name {
			return nil, nil
		}
	}

	// Pattern to match H1 heading
	h1Pattern := regexp.MustCompile(`(?m)^# (.+)$`)
	matches := h1Pattern.FindStringSubmatchIndex(doc.Content)

	if matches != nil {
		// Check for text before H1
		textBeforeH1 := strings.TrimSpace(doc.Content[:matches[0]])
		if textBeforeH1 == "" {
			// Extract title from H1
			title := strings.TrimSpace(doc.Content[matches[2]:matches[3]])
			if title != "" {
				doc.FrontMatter["title"] = title
			}
			return nil, nil
		}
	}

	return nil, nil
}

// stripHeading removes the first H1 heading from content when a title exists in frontmatter.
// Rules:
// - If title exists in frontmatter → strip first H1 at start of content (if present)
// - This works in conjunction with extractH1AsTitle which extracts H1 as title when needed
func stripHeading(doc *Document) ([]*Document, error) {
	// Only strip if we have a title in frontmatter
	_, hasTitle := doc.FrontMatter["title"].(string)
	if !hasTitle {
		return nil, nil
	}

	// Pattern to match H1 heading at start of content
	h1Pattern := regexp.MustCompile(`(?m)^# (.+)\n?`)
	
	// Check if H1 is at the start of content (no text before it)
	matches := h1Pattern.FindStringSubmatchIndex(doc.Content)
	if matches != nil {
		textBeforeH1 := strings.TrimSpace(doc.Content[:matches[0]])
		if textBeforeH1 == "" {
			// Strip the first H1 since we have a title in frontmatter
			doc.Content = h1Pattern.ReplaceAllString(doc.Content, "")
		}
	}

	return nil, nil
}
