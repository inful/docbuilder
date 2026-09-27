package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
)

// applyDirectoryCategoryFixes injects the expected category (per the
// directory-category-consistency rule's map) into a doc's `categories:`
// list when missing. Categories are not auto-replaced if they exist but
// are wrong — that's a user decision (renaming a category changes URLs).
func (f *Fixer) applyDirectoryCategoryFixes(targets map[string]struct{}, issueCounts map[string]int, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
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

		expected, ok := directoryToCategory[filepath.Base(filepath.Dir(p))]
		if !ok {
			// No expected category for this directory; nothing to do.
			continue
		}

		op := f.injectCategory(p, expected)
		if op.Success {
			fixResult.CategoryInjects = append(fixResult.CategoryInjects, op)
			fixResult.ErrorsFixed += issueCounts[p]
			fingerprintTargets[p] = struct{}{}
			continue
		}
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
		}
	}
}

// CategoryInjectUpdate records a directory-category-consistency fix.
type CategoryInjectUpdate struct {
	FilePath  string
	Category  string
	Success   bool
	Error     error
}

// injectCategory appends the given category to a doc's `categories:`
// list if not already present. If `categories:` is missing entirely,
// it is added with the single injected value.
func (f *Fixer) injectCategory(filePath, category string) CategoryInjectUpdate {
	op := CategoryInjectUpdate{FilePath: filePath, Category: category, Success: true}

	// #nosec G304 -- filePath is derived from the current lint/fix target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for category inject: %w", err)
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

	if !frontmatterops.EnsureCategory(fields, category) {
		// Already present or not applicable; no change.
		return op
	}

	if !had {
		had = true
		if len(body) > 0 && !bytesHasPrefixNewline(body, style.Newline) {
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
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for category inject: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, updated, info.Mode().Perm()); writeErr != nil { //nolint:gosec // filePath is derived from the lint target set
		op.Success = false
		op.Error = fmt.Errorf("write file for category inject: %w", writeErr)
		return op
	}

	return op
}

// bytesHasPrefixNewline reports whether b starts with the given newline
// sequence (handles both "\n" and "\r\n").
func bytesHasPrefixNewline(b []byte, nl string) bool {
	if len(nl) == 0 {
		return false
	}
	if len(b) < len(nl) {
		return false
	}
	return string(b[:len(nl)]) == nl
}
