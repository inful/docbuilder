package pipeline

import (
	"regexp"
	"strings"
)

// extractIndexTitle extracts H1 heading from index files to use as title.
// Only processes index files and only if H1 is at the start of content.
// If no H1 is found and this is a docs base section, uses repository name as title.
func extractIndexTitle(doc *Document) ([]*Document, error) {
	if !doc.IsIndex {
		return nil, nil // Only process index files
	}

	// Skip if title already exists
	if _, hasTitle := doc.FrontMatter["title"]; hasTitle {
		return nil, nil
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

	// No H1 found or text before H1 - check if this is a docs base section
	// If section matches DocsBase, this is a repository-level docs directory
	// Use repository name as title instead of section name
	if doc.Section != "" && doc.DocsBase == doc.Section && doc.Repository != "" {
		repoTitle := titleCase(doc.Repository)
		if repoTitle != "" {
			doc.FrontMatter["title"] = repoTitle
		}
		return nil, nil
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

	// Only strip if H1 matches front matter title
	if h1Title == fmTitle {
		doc.Content = h1Pattern.ReplaceAllString(doc.Content, "")
	}

	return nil, nil
}
