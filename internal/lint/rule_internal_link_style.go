package lint

import (
	"os"
	"path/filepath"
	"regexp"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// InternalLinkStyleRule warns when internal markdown links are not in
// the canonical `.md` form. Specifically:
//   - `[text](relative/path)` without `.md` extension → WARNING
//   - `[text](/site-rooted)` (absolute path starting with `/`) → WARNING
//
// External URLs (`http://`, `https://`) and in-page anchors (`#foo`)
// are not flagged.
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
	for _, line := range splitLines(string(body)) {
		matches := linkRe.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			target := m[1]
			if isExternal(target) || isAnchor(target) {
				continue
			}
			if isSiteRooted(target) {
				issues = append(issues, r.siteRootedIssue(filePath, target, lineNum))
				continue
			}
			if hasExplicitExtension(target) && !hasMarkdownExtension(target) {
				continue
			}
			if !hasMarkdownExtension(target) {
				issues = append(issues, r.missingExtIssue(filePath, target, lineNum))
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

func hasMarkdownExtension(target string) bool {
	// Strip query string and fragment before checking extension
	t := stripQueryAndFragment(target)
	return len(t) >= 3 && (t[len(t)-3:] == ".md" || (len(t) >= 9 && t[len(t)-9:] == ".markdown"))
}

func hasExplicitExtension(target string) bool {
	return filepath.Ext(stripQueryAndFragment(target)) != ""
}

func stripQueryAndFragment(target string) string {
	t := target
	for i := 0; i < len(t); i++ {
		if t[i] == '?' || t[i] == '#' {
			return t[:i]
		}
	}
	return t
}

func (r *InternalLinkStyleRule) missingExtIssue(filePath, target string, line int) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityWarning,
		Rule:     r.Name(),
		Message:  "Internal link missing .md extension",
		Explanation: "Internal link target `" + target + "` doesn't end in `.md`. " +
			"Docbuilder's broken-link detector only walks the `.md` form; standardizing " +
			"on the canonical extension makes link checking uniform.",
		Fix:  "Append `.md` to the link target: `(" + target + ".md)`.",
		Line: line,
	}
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
