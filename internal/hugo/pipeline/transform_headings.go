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

// stripHeading removes the first H1 heading from content if appropriate.
// Only strips if H1 matches the title in front matter.
func stripHeading(doc *Document) ([]*Document, error) {
	// Check if we have a title in front matter
	title, hasTitle := doc.FrontMatter["title"].(string)
	if !hasTitle {
		return nil, nil
	}

	// Pattern to match H1 heading
	h1Pattern := regexp.MustCompile(`(?m)^# (.+)\n?`)
	matches := h1Pattern.FindStringSubmatch(doc.Content)
	if matches == nil {
		return nil, nil // No H1 found
	}

	h1Title := strings.TrimSpace(matches[1])
	fmTitle := strings.TrimSpace(title)

	// Strip if H1 matches front matter title (exact or case-insensitive match)
	// or if H1 starts with the front matter title (common pattern: title + additional context)
	h1Lower := strings.ToLower(h1Title)
	fmLower := strings.ToLower(fmTitle)

	if h1Lower == fmLower || strings.HasPrefix(h1Lower, fmLower) {
		doc.Content = h1Pattern.ReplaceAllString(doc.Content, "")
	}

	return nil, nil
}
