package pipeline

import (
	"strings"
)

// generateFromKeywords scans for special keywords and generates related files.
// Example: @glossary tag creates a glossary page from all terms.
func generateFromKeywords(doc *Document) ([]*Document, error) {
	// Skip generated documents (prevent infinite loops)
	if doc.Generated {
		return nil, nil
	}

	var newDocs []*Document

	// Check for @glossary marker (placeholder - would need actual implementation)
	if strings.Contains(doc.Content, "@glossary") {
		// For now, just remove the marker
		doc.Content = strings.ReplaceAll(doc.Content, "@glossary", "")
		// TODO: Implement actual glossary generation
	}

	return newDocs, nil
}
