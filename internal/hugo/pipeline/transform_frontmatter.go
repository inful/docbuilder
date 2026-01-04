package pipeline

import (
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const untitledDocTitle = "Untitled"

// parseFrontMatter extracts YAML front matter from content.
// Sets OriginalFrontMatter and removes front matter from Content.
// Idempotent: if already parsed (OriginalFrontMatter is set), does nothing.
func parseFrontMatter(doc *Document) ([]*Document, error) {
	// Idempotence check: if we've already parsed front matter, don't re-parse
	if doc.OriginalFrontMatter != nil {
		return nil, nil
	}

	content := doc.Content

	// Check for YAML front matter (--- ... ---)
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		// No front matter
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		doc.FrontMatter = make(map[string]any)
		return nil, nil
	}

	// Determine line ending
	var lineEnd string
	var startLen int
	if strings.HasPrefix(content, "---\r\n") {
		lineEnd = "\r\n"
		startLen = 5
	} else {
		lineEnd = "\n"
		startLen = 4
	}

	// Find end of front matter (search for closing ---\n or ---\r\n)
	endMarker := lineEnd + "---" + lineEnd
	endIdx := strings.Index(content[startLen:], endMarker)

	if endIdx == -1 {
		// Try to find just "---" followed by line ending (for content like "---\n---\n...")
		altMarker := "---" + lineEnd
		endIdx = strings.Index(content[startLen:], altMarker)
		if endIdx != -1 {
			// Adjust for the different marker length
			endMarker = altMarker
		}
	}

	if endIdx == -1 {
		// Malformed front matter - no closing delimiter
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		doc.FrontMatter = make(map[string]any)
		return nil, nil
	}

	// Extract front matter YAML
	fmYAML := content[startLen : startLen+endIdx]
	bodyStart := startLen + endIdx + len(endMarker)

	// Always remove front matter delimiters from content, even if empty
	doc.Content = content[bodyStart:]

	// Parse YAML (handle empty front matter)
	if strings.TrimSpace(fmYAML) == "" {
		// Empty front matter - no fields but delimiters were present
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		doc.FrontMatter = make(map[string]any)
		return nil, nil
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(fmYAML), &fm); err != nil {
		// Invalid YAML - treat as no front matter but content already stripped
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		doc.FrontMatter = make(map[string]any)
		return nil, nil //nolint:nilerr // Intentionally suppressing YAML parse error to gracefully handle malformed frontmatter
	}

	doc.HadFrontMatter = true
	doc.OriginalFrontMatter = fm
	// Deep copy to FrontMatter (transforms will modify this)
	doc.FrontMatter = deepCopyMap(fm)

	return nil, nil
}

// buildBaseFrontMatter initializes FrontMatter with Hugo-compatible base fields.
func buildBaseFrontMatter(doc *Document) ([]*Document, error) {
	if doc.FrontMatter == nil {
		doc.FrontMatter = make(map[string]any)
	}

	// Always set title if not present
	if _, hasTitle := doc.FrontMatter["title"]; !hasTitle {
		// Use the filename (without extension) as title
		if doc.Name != "" {
			doc.FrontMatter["title"] = doc.Name
		} else {
			// Fallback to "Untitled" if name is empty
			doc.FrontMatter["title"] = untitledDocTitle
		}
	}

	// Ensure title is never empty (safety check)
	if title, ok := doc.FrontMatter["title"].(string); ok && strings.TrimSpace(title) == "" {
		if doc.Name != "" {
			doc.FrontMatter["title"] = doc.Name
		} else {
			doc.FrontMatter["title"] = "Untitled"
		}
	}

	// Set type=docs for Hextra theme
	if _, hasType := doc.FrontMatter["type"]; !hasType {
		doc.FrontMatter["type"] = "docs"
	}

	// Add date if not present (required by Hugo for proper sorting/display)
	// Use git commit date if available, otherwise fall back to current time
	if _, hasDate := doc.FrontMatter["date"]; !hasDate {
		var dateStr string
		if !doc.CommitDate.IsZero() {
			dateStr = doc.CommitDate.Format("2006-01-02T15:04:05-07:00")
		} else {
			dateStr = time.Now().Format("2006-01-02T15:04:05-07:00")
		}
		doc.FrontMatter["date"] = dateStr
	}

	// Add edit link for non-index files
	if doc.SourceURL != "" && doc.SourceBranch != "" && doc.RelativePath != "" {
		if _, hasEditURL := doc.FrontMatter["editURL"]; !hasEditURL {
			editURL := generateEditURL(doc)
			if editURL != "" {
				doc.FrontMatter["editURL"] = editURL
			}
		}
	}

	return nil, nil
}

// serializeDocument converts the Document back to markdown with front matter.
// Idempotent: if already serialized (Raw is set), does nothing.
func serializeDocument(doc *Document) ([]*Document, error) {
	// Idempotence check: if already serialized, don't re-serialize
	if len(doc.Raw) > 0 {
		return nil, nil
	}

	var result strings.Builder

	// Write front matter if present
	if len(doc.FrontMatter) > 0 {
		result.WriteString("---\n")
		yamlData, err := yaml.Marshal(doc.FrontMatter)
		if err != nil {
			return nil, err
		}
		result.Write(yamlData)
		result.WriteString("---\n")
	}

	// Write content
	result.WriteString(doc.Content)

	// Update both Content and Raw
	doc.Content = result.String()
	doc.Raw = []byte(doc.Content)

	return nil, nil
}
