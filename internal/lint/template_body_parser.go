package lint

import (
	"regexp"
	"strings"
)

// templateFenceOpen matches the opening of a fenced code block whose
// language is `markdown` or `md`. The fence may be 3+ backticks, and the
// language may be followed by attributes (e.g., `markdown {.class}`).
//
// Examples that match:
//   - ```markdown
//   - ```md
//   - ````markdown
var templateFenceOpen = regexp.MustCompile("^(`{3,})(?:markdown|md)\\b[^\\n]*$")

// templateFenceClose matches a closing fence. It must be at least as many
// backticks as the opening fence and at the start of a line.
var templateFenceClose = regexp.MustCompile("^(`{3,})\\s*$")

// extractTemplateBody returns the content of the first markdown/md
// fenced code block in the given markdown text, plus the byte range
// (start, end) within the original text where the body sits.
//
// Returns ok=false if no such fence is found. The body is the substring
// BETWEEN the opening and closing fences, with no leading or trailing
// newline.
//
// The returned offsets are byte offsets in the original text:
//   - start = first byte of the body content
//   - end   = byte just past the last body byte (so text[start:end] == body)
//
// The offsets are chosen so that callers can splice the body back in
// after rewriting it: newContent = text[:start] + newBody + text[end:].
func extractTemplateBody(text string) (body string, start, end int, ok bool) {
	lines := strings.Split(text, "\n")

	bodyStartLine := -1
	bodyEndLine := -1
	fenceLen := 0

	for i, line := range lines {
		if bodyStartLine == -1 {
			m := templateFenceOpen.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			bodyStartLine = i + 1
			fenceLen = len(m[1])
			continue
		}
		if templateFenceClose.MatchString(line) && len(line) >= fenceLen {
			// Confirmed close: the line is backticks-only (whitespace OK)
			// and at least as long as the opener. Verify the opening
			// length matches.
			prefix := strings.Repeat("`", fenceLen)
			if strings.HasPrefix(line, prefix) && strings.TrimSpace(line[len(prefix):]) == "" {
				bodyEndLine = i
				break
			}
		}
	}

	if bodyStartLine == -1 || bodyEndLine == -1 {
		return "", 0, 0, false
	}

	// Compute byte offsets for the body content.
	// start = byte just past the newline that terminates the opening fence line.
	prefix := strings.Join(lines[:bodyStartLine], "\n")
	start = len(prefix)
	if bodyStartLine > 0 {
		start++ // include the newline separator after the opening line
	}

	bodyLines := lines[bodyStartLine:bodyEndLine]
	body = strings.Join(bodyLines, "\n")
	end = start + len(body)

	return body, start, end, true
}

// hasUIDReference reports whether s contains a Go text/template
// reference to the .UID field. Matches both `{{ .UID }}` and
// `{{- .UID -}}` and the more exotic spacing variants.
var uidRefPattern = regexp.MustCompile(`\{\{[\s\-]*\.UID[\s\-]*(\|[^}]*)?\}\}`)

func hasUIDReference(s string) bool {
	return uidRefPattern.MatchString(s)
}

// uidFieldIsTemplate reports whether the frontmatter's `uid` field is a
// Go text/template reference to .UID (i.e., it would render the new
// document's UID at template time).
func uidFieldIsTemplate(uidValue string) bool {
	return uidRefPattern.MatchString(uidValue)
}
