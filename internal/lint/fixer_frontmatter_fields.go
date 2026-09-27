package lint

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
)

// applyFrontmatterFieldsFixes fills missing required fields in frontmatter.
// Specifically:
//
//   - title: extracted from the body's first H1 if missing; otherwise
//     derived from the file's basename
//   - date: today's UTC date in RFC 3339
//   - lastmod: today's UTC date in YYYY-MM-DD
//   - tags: empty list (`[]`) — preserves the "may be empty" semantics
//
// `categories` is intentionally NOT auto-filled because the choice of
// category is a user decision (semantic).
func (f *Fixer) applyFrontmatterFieldsFixes(targets map[string]struct{}, issueCounts map[string]int, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
	if len(targets) == 0 {
		return
	}

	paths := make([]string, 0, len(targets))
	for p := range targets {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		ext := strings.ToLower(filepath.Ext(p))
		if ext != docExtensionMarkdown && ext != docExtensionMarkdownLong {
			continue
		}

		op := f.fillFrontmatterFields(p)
		if op.Success {
			fixResult.FrontmatterFields = append(fixResult.FrontmatterFields, op)
			fixResult.ErrorsFixed += issueCounts[p]
			// Frontmatter content changes invalidate fingerprints.
			fingerprintTargets[p] = struct{}{}
			continue
		}
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
		}
	}
}

// FrontmatterFieldsUpdate represents a frontmatter-fields fix operation.
type FrontmatterFieldsUpdate struct {
	FilePath string
	Fields   []string // which fields were filled in
	Success  bool
	Error    error
}

var bodyH1Regex = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)

// fillFrontmatterFields reads a markdown file and adds any missing
// required fields (`title`, `date`, `lastmod`, `tags`) without
// disturbing other content. Categories are intentionally NOT auto-added
// (see rule_frontmatter_required_fields.go for rationale).
func (f *Fixer) fillFrontmatterFields(filePath string) FrontmatterFieldsUpdate {
	op := FrontmatterFieldsUpdate{FilePath: filePath, Success: true}

	// #nosec G304 -- filePath is derived from the current lint/fix target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for frontmatter-fields update: %w", err)
		return op
	}

	fields, body, had, style, readErr := frontmatterops.Read(data)
	if readErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("read frontmatter: %w", readErr)
		return op
	}
	if style.Newline == "" {
		style.Newline = "\n"
	}
	if fields == nil {
		fields = map[string]any{}
	}

	now := f.now()
	var filled []string

	// title: extract from body's first H1, or fall back to filename stem.
	if _, ok := fields["title"]; !ok {
		title := extractTitleFromBody(body, filePath)
		if title != "" {
			fields["title"] = title
			filled = append(filled, "title")
		}
	}

	// date / lastmod: today's date.
	if _, ok := fields["date"]; !ok {
		if frontmatterops.EnsureDate(fields, time.Time{}, now) {
			filled = append(filled, "date")
		}
	}
	if _, ok := fields["lastmod"]; !ok || fields["lastmod"] == "" {
		if frontmatterops.EnsureLastmod(fields, time.Time{}, now) {
			filled = append(filled, "lastmod")
		}
	}

	// tags: empty list (the lint rule allows empty tags).
	if _, ok := fields["tags"]; !ok {
		if frontmatterops.EnsureTags(fields) {
			filled = append(filled, "tags")
		}
	}

	if len(filled) == 0 {
		// Nothing to fix; report success but no operation.
		return op
	}

	// Ensure body has a leading newline if frontmatter is added.
	if !had {
		had = true
		if len(body) > 0 && !bytes.HasPrefix(body, []byte(style.Newline)) {
			body = append([]byte(style.Newline), body...)
		} else if len(body) == 0 {
			body = append([]byte(style.Newline), body...)
		}
	}

	updated, writeErr := frontmatterops.Write(fields, body, had, style)
	if writeErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("write frontmatter: %w", writeErr)
		return op
	}

	if f.dryRun {
		op.Fields = filled
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for frontmatter-fields update: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, updated, info.Mode().Perm()); writeErr != nil { //nolint:gosec // filePath is derived from the lint target set
		op.Success = false
		op.Error = fmt.Errorf("write file for frontmatter-fields update: %w", writeErr)
		return op
	}

	op.Fields = filled
	return op
}

// extractTitleFromBody returns the first H1's text from body, or the
// file's basename (with separators replaced) as a fallback.
func extractTitleFromBody(body []byte, filePath string) string {
	if m := bodyH1Regex.FindSubmatch(body); m != nil {
		title := strings.TrimSpace(string(m[1]))
		// Strip surrounding quotes the user may have written in the H1.
		title = strings.Trim(title, `"'`)
		if title != "" {
			return title
		}
	}
	base := filepath.Base(filePath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	// kebab-case to Title Case: split on '-' and capitalise.
	parts := strings.Split(base, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// now returns the current time in UTC. Uses f.nowFn so tests can
// inject a fixed clock if needed; defaults to time.Now.
func (f *Fixer) now() time.Time {
	if f.nowFn != nil {
		return f.nowFn()
	}
	return time.Now()
}
