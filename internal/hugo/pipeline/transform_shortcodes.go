package pipeline

import (
	"strings"
)

// escapeShortcodesInCodeBlocks escapes Hugo shortcodes within markdown code blocks
// to prevent Hugo from trying to process them as actual shortcodes.
// This is essential for documentation that explains or demonstrates shortcode usage.
func escapeShortcodesInCodeBlocks(doc *Document) ([]*Document, error) {
	// Pattern to match code blocks (both ``` and indented)
	// We need to find {{< shortcode >}} and {{< /shortcode >}} patterns within code blocks

	var result strings.Builder
	lines := strings.Split(doc.Content, "\n")
	inFencedCodeBlock := false
	var fenceMarker string

	for _, line := range lines {
		// Check for fenced code block markers (```  or ~~~)
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "```") || strings.HasPrefix(trimmedLine, "~~~") {
			if !inFencedCodeBlock {
				// Starting a code block
				inFencedCodeBlock = true
				if strings.HasPrefix(trimmedLine, "```") {
					fenceMarker = "```"
				} else {
					fenceMarker = "~~~"
				}
			} else if strings.HasPrefix(trimmedLine, fenceMarker) {
				// Ending a code block
				inFencedCodeBlock = false
			}
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// If we're in a code block, escape shortcodes (but skip already-escaped ones)
		if inFencedCodeBlock {
			// Check if shortcodes are already escaped (contains {{</* or */>}})
			switch {
			case strings.Contains(line, "{{</*") || strings.Contains(line, "*/>}}"):
				// Already escaped, leave as-is
				result.WriteString(line)
			case strings.Contains(line, "{{<"):
				// Only escape if line contains shortcode opening {{<
				// Escape both opening {{< and closing >}}
				escapedLine := strings.ReplaceAll(line, "{{<", "{{</*")
				escapedLine = strings.ReplaceAll(escapedLine, ">}}", "*/>}}")
				result.WriteString(escapedLine)
			default:
				// No shortcodes to escape
				result.WriteString(line)
			}
		} else {
			result.WriteString(line)
		}
		result.WriteString("\n")
	}

	// Remove trailing newline if original didn't have one
	content := result.String()
	if !strings.HasSuffix(doc.Content, "\n") && strings.HasSuffix(content, "\n") {
		content = strings.TrimSuffix(content, "\n")
	}

	doc.Content = content
	// This transform only modifies content, doesn't generate new documents
	return nil, nil
}
