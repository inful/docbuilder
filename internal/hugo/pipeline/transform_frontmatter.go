package pipeline

import (
	"bytes"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
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

	fmRaw, body, had, _, err := frontmatter.Split([]byte(doc.Content))
	if err != nil {
		// Malformed front matter (missing closing delimiter): treat as no front matter
		// and do not modify content.
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		// Preserve any pre-populated frontmatter (e.g., from generators).
		if len(doc.FrontMatter) == 0 {
			doc.FrontMatter = make(map[string]any)
		}
		return nil, nil
	}
	if !had {
		// No front matter
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		// Preserve any pre-populated frontmatter (e.g., from generators).
		if len(doc.FrontMatter) == 0 {
			doc.FrontMatter = make(map[string]any)
		}
		return nil, nil
	}

	// Always remove front matter delimiters from content, even if empty/invalid.
	doc.Content = string(body)

	if len(bytes.TrimSpace(fmRaw)) == 0 {
		// Empty front matter - no fields but delimiters were present.
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		// Preserve any pre-populated frontmatter (e.g., from generators).
		if len(doc.FrontMatter) == 0 {
			doc.FrontMatter = make(map[string]any)
		}
		return nil, nil
	}

	fm, err := frontmatter.ParseYAML(fmRaw)
	if err != nil {
		// Invalid YAML - treat as no front matter but content already stripped.
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		// Preserve any pre-populated frontmatter (e.g., from generators).
		if len(doc.FrontMatter) == 0 {
			doc.FrontMatter = make(map[string]any)
		}
		return nil, nil
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
		switch {
		case doc.IsIndex:
			// For indices, we might extract title from H1 later (extractIndexTitle).
			// If name is present and not just "index", it's a good fallback.
			if doc.Name != "" && doc.Name != "index" && doc.Name != "_index" {
				doc.FrontMatter["title"] = formatTitle(doc.Name)
			}
		case doc.Name != "":
			doc.FrontMatter["title"] = formatTitle(doc.Name)
		default:
			doc.FrontMatter["title"] = untitledDocTitle
		}
	}

	// Ensure title is never empty (safety check)
	if title, ok := doc.FrontMatter["title"].(string); ok && strings.TrimSpace(title) == "" {
		if doc.Name != "" {
			doc.FrontMatter["title"] = formatTitle(doc.Name)
		} else {
			doc.FrontMatter["title"] = untitledDocTitle
		}
	}

	// Set type=docs for Relearn theme (ensures proper layout)
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

	return nil, nil
}

// formatTitle converts kebab-case or snake_case to Title Case.
func formatTitle(name string) string {
	base := strings.ReplaceAll(name, "_", "-")
	parts := strings.Split(base, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

// serializeDocument converts the Document back to markdown with front matter.
// Idempotent: if already serialized (Raw is set), does nothing.
func serializeDocument(doc *Document) ([]*Document, error) {
	// Idempotence check: if already serialized, don't re-serialize
	if len(doc.Raw) > 0 {
		return nil, nil
	}

	// Pipeline output uses LF newlines.
	style := frontmatter.Style{Newline: "\n"}
	had := len(doc.FrontMatter) > 0

	fmYAML, err := frontmatter.SerializeYAML(doc.FrontMatter, style)
	if err != nil {
		return nil, err
	}

	out := frontmatter.Join(fmYAML, []byte(doc.Content), had, style)

	// Update both Content and Raw
	doc.Content = string(out)
	doc.Raw = out

	return nil, nil
}
