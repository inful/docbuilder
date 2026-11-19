package lint

import (
	"os"
	"regexp"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// InternalLinkStyleRule warns when internal markdown links are
// site-rooted (start with `/`). Cross-renderer portability is the
// concern: a link like `[Other](/other-page)` works in Hugo but
// produces dead links in any renderer that doesn't serve from the
// site root.
//
// External URLs (`http://`, `https://`) and in-page anchors (`#foo`)
// are not flagged.
//
// The rule used to also flag relative links without `.md` extension,
// but that warning conflicted with Hugo's documented directory-link
// pattern (`[text](./api/)` resolving to `api/_index.md`) and the
// matching auto-fix actively broke valid directory links. The
// broken-link detector covers non-existence with the correct `.md`
// fallback, so the extension-style warning was redundant.
type InternalLinkStyleRule struct{}

// Name returns the rule identifier.
func (r *InternalLinkStyleRule) Name() string {
	return ruleInternalLinkStyle
}

// AppliesTo returns true for documentation files.
func (r *InternalLinkStyleRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// linkRe captures markdown inline links [text](target).
var linkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// Check reads the body and flags non-canonical internal links.
func (r *InternalLinkStyleRule) Check(filePath string) ([]Issue, error) {
	// #nosec G304 -- filePath is from controlled lint walk.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	_, body, _, _, splitErr := frontmatter.Split(data)
	if splitErr != nil {
		//nolint:nilerr // split errors are reported by other rules.
		return nil, nil
	}

	var issues []Issue
	lineNum := 1
	inFence := ""
	for _, line := range splitLines(string(body)) {
		// A link inside a code block is an example, not a link in the
		// document. The how-to guides show what a site-rooted link
		// looks like, and the rule must not flag their own examples.
		if marker := codeFenceMarker(strings.TrimSpace(line)); marker != "" {
			if inFence == "" {
				inFence = marker
			} else if strings.HasPrefix(marker, inFence) {
				inFence = ""
			}
			lineNum++
			continue
		}
		if inFence != "" {
			lineNum++
			continue
		}

		matches := linkRe.FindAllStringSubmatch(stripInlineCode(line), -1)
		for _, m := range matches {
			target := m[1]
			if isExternal(target) || isAnchor(target) {
				continue
			}
			if isSiteRooted(target) {
				issues = append(issues, r.siteRootedIssue(filePath, target, lineNum))
			}
		}
		lineNum++
	}
	return issues, nil
}

// splitLines splits on LF and counts newlines as line separators.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, ch := range s {
		if ch == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// stripInlineCode removes the inline code spans from a line. A link
// inside backticks is an example, not a link in the document.
func stripInlineCode(line string) string {
	var b strings.Builder
	inCode := false
	for _, r := range line {
		if r == '`' {
			inCode = !inCode
			continue
		}
		if !inCode {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// codeFenceMarker returns the fence of a markdown code fence line ("```" or
// "~~~"), or "" when the line does not open or close a fence.
func codeFenceMarker(line string) string {
	for _, marker := range []string{"```", "~~~"} {
		if strings.HasPrefix(line, marker) {
			return marker
		}
	}
	return ""
}

func isExternal(target string) bool {
	if len(target) >= 8 && target[:8] == "https://" {
		return true
	}
	if len(target) >= 7 && target[:7] == "http://" {
		return true
	}
	return false
}

func isAnchor(target string) bool {
	return len(target) > 0 && target[0] == '#'
}

func isSiteRooted(target string) bool {
	return len(target) > 0 && target[0] == '/'
}

func (r *InternalLinkStyleRule) siteRootedIssue(filePath, target string, line int) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityWarning,
		Rule:     r.Name(),
		Message:  "Site-rooted link — use a relative path",
		Explanation: "Link target `" + target + "` starts with `/` (site-rooted). " +
			"Docbuilder's broken-link detector only checks relative `.md` paths; " +
			"use a relative path so the link is portable across renderers.",
		Fix:  "Rewrite as a relative `.md` path: use `../foo.md` (or `foo.md`) instead.",
		Line: line,
	}
}
