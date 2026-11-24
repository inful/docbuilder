package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
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
	FilePath string
	Category string
	Success  bool
	Error    error
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

	// The categories of a doc are its doc mode: the wrong one is
	// replaced, not joined. A doc with two modes is an error that no
	// fixer can remove.
	if !frontmatterops.SetCategory(fields, category) {
		// Already the expected category; no change.
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

	// The categories are patched into the raw frontmatter: the
	// comments of the file and the order of its keys survive the fix
	// (ADR-014).
	updated, ok := frontmatterops.PatchFrontmatter(data, body, had, style, map[string]any{"categories": fields["categories"]})
	if !ok {
		op.Success = false
		op.Error = fmt.Errorf("write frontmatter: the frontmatter of %s could not be patched", filePath)
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

	if writeErr := foundation.WriteFileAtomic(filePath, updated, info.Mode().Perm()); writeErr != nil {
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
