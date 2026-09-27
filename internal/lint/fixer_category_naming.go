package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
)

// applyCategoryNamingFixes converts non-kebab-case categories to
// kebab-case. Lowercases the first character and replaces underscores
// with hyphens. Each category is converted independently.
//
// Note: renaming a category changes Hugo URLs (taxonomy is part of the
// rendered URL path on Hugo). The rule that triggers this fixer
// (#71, category-naming) is ERROR severity but the fix is potentially
// URL-impacting, so the fixer changes the field in-place rather than
// re-architecting. Operators who care about preserving old URLs can
// add aliases via the existing frontmatter-ops.
func (f *Fixer) applyCategoryNamingFixes(targets map[string]struct{}, issueCounts map[string]int, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
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

		op := f.kebabCaseCategories(p)
		if op.Success {
			fixResult.CategoryRenames = append(fixResult.CategoryRenames, op)
			fixResult.ErrorsFixed += issueCounts[p]
			fingerprintTargets[p] = struct{}{}
			continue
		}
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
		}
	}
}

// CategoryRenameUpdate records a category-naming fix.
type CategoryRenameUpdate struct {
	FilePath string
	Renamed  map[string]string // old → new (only successful conversions)
	Success  bool
	Error    error
}

var lowerFirstChar = regexp.MustCompile(`^[A-Z]`)

// kebabCaseCategories rewrites the categories field so each value
// matches `^[a-z][a-z0-9-]*$`. Categories already matching are skipped.
// Returns the renamed map so callers can update cross-references if
// desired.
func (f *Fixer) kebabCaseCategories(filePath string) CategoryRenameUpdate {
	op := CategoryRenameUpdate{FilePath: filePath, Success: true, Renamed: map[string]string{}}

	// #nosec G304 -- filePath is derived from the current lint/fix target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for category-naming fix: %w", err)
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

	raw, ok := fields["categories"]
	if !ok || raw == nil {
		// Nothing to fix; should not happen if linter ran correctly.
		return op
	}

	changed := false
	switch v := raw.(type) {
	case []any:
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			k := toKebabCaseString(s)
			if k != s && categoryPattern.MatchString(k) {
				op.Renamed[s] = k
				v[i] = k
				changed = true
			}
		}
		fields["categories"] = v
	case []string:
		for i, s := range v {
			k := toKebabCaseString(s)
			if k != s && categoryPattern.MatchString(k) {
				op.Renamed[s] = k
				v[i] = k
				changed = true
			}
		}
		fields["categories"] = v
	}

	if !changed {
		return op
	}

	updated, writeErr := frontmatterops.Write(fields, body, had, style)
	if writeErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("write frontmatter: %w", writeErr)
		return op
	}

	if f.dryRun {
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for category-naming fix: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, updated, info.Mode().Perm()); writeErr != nil { //nolint:gosec // filePath is derived from the lint target set
		op.Success = false
		op.Error = fmt.Errorf("write file for category-naming fix: %w", writeErr)
		return op
	}

	return op
}

// toKebabCaseString converts a category string to kebab-case:
//   - lowercases the first character
//   - replaces underscores with hyphens
//
// This matches the existing toKebabCase helper used by the rule's
// explanation text; duplicated here to avoid an export dependency.
func toKebabCaseString(s string) string {
	if s == "" {
		return s
	}
	if lowerFirstChar.MatchString(s) {
		s = strings.ToLower(s[:1]) + s[1:]
	}
	return strings.ReplaceAll(s, "_", "-")
}
