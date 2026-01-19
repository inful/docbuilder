package hugo

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// injectUIDPermalink inspects the Markdown content's YAML frontmatter for a
// non-empty "uid" field and a matching "/_uid/<uid>/" value in "aliases". When both are
// present and no existing permalink line is found, it appends a plain Markdown permalink
// line using the UID alias at the end of the content.
//
// The content parameter is the full Markdown file contents including frontmatter.
// It returns the potentially updated content string and a boolean indicating whether
// a permalink line was injected.
func injectUIDPermalink(content string) (string, bool) {
	fm, ok := parseYAMLFrontMatter(content)
	if !ok || fm == nil {
		return content, false
	}

	uid, _ := fm["uid"].(string)
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return content, false
	}

	aliasWant := "/_uid/" + uid + "/"
	if !frontMatterHasAlias(fm, aliasWant) {
		return content, false
	}

	// NOTE: Hugo's ref/relref does not resolve aliases (they are redirect outputs, not pages),
	// so linking via ref to /_uid/<uid>/ breaks real Hugo renders with REF_NOT_FOUND.
	// Use a plain link to the stable alias instead.
	permalinkLineBadge := fmt.Sprintf(`{{%% badge style="note" title="permalink" %%}}`+"`%s`"+`{{%% /badge %%}}`, aliasWant)

	// Idempotence: don't add again if already present (either format).
	// We check for legacy plain and ref formats as well.
	permalinkLinePlain := fmt.Sprintf(`[Permalink](%s)`, aliasWant)
	permalinkLineRef := fmt.Sprintf(`[Permalink]({{%% ref "%s" %%}})`, aliasWant)

	if strings.Contains(content, permalinkLineBadge) ||
		strings.Contains(content, permalinkLinePlain) ||
		strings.Contains(content, permalinkLineRef) {
		return content, false
	}

	trimmed := strings.TrimRight(content, "\r\n")
	updated := trimmed + "\n\n" + permalinkLineBadge + "\n"
	return updated, true
}

// frontMatterHasAlias reports whether the front matter "aliases" field contains
// the given alias value, handling both string and slice (array) formats.
func frontMatterHasAlias(fm map[string]any, want string) bool {
	v, exists := fm["aliases"]
	if !exists || v == nil {
		return false
	}

	// Common shapes:
	// aliases: "/path" (string)
	// aliases: ["/path"] ([]any / []string)
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) == want
	case []string:
		for _, s := range t {
			if strings.TrimSpace(s) == want {
				return true
			}
		}
		return false
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok {
				if strings.TrimSpace(s) == want {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// parseYAMLFrontMatter extracts and parses the leading YAML frontmatter block
// from markdown content, handling both LF and CRLF line endings for the
// `---` delimiters. It returns the parsed frontmatter and a boolean
// indicating whether a valid frontmatter block was found and parsed.
func parseYAMLFrontMatter(content string) (map[string]any, bool) {
	// Support both LF and CRLF. Hugo frontmatter for markdown uses --- delimiters.
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return nil, false
	}

	lineEnd := "\n"
	startLen := 4
	if strings.HasPrefix(content, "---\r\n") {
		lineEnd = "\r\n"
		startLen = 5
	}

	endMarker := lineEnd + "---" + lineEnd
	endIdx := strings.Index(content[startLen:], endMarker)
	if endIdx == -1 {
		// Malformed or empty frontmatter.
		return nil, false
	}

	fmYAML := content[startLen : startLen+endIdx]
	if strings.TrimSpace(fmYAML) == "" {
		return map[string]any{}, true
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(fmYAML), &fm); err != nil {
		return nil, false
	}
	return fm, true
}
