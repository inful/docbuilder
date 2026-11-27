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

// applyWebSourcesFixes turns a lone web_sources string into a list of
// one.
//
// The entries of the list are left alone: an entry that is not an
// http(s) URL is the author's decision. The fixer does not guess a
// scheme and does not drop a page the author asked for, so the lint
// keeps reporting those entries until the author writes them properly.
func (f *Fixer) applyWebSourcesFixes(targets map[string]struct{}, issueCounts map[string]int, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
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

		op := f.normalizeWebSources(p)
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
			continue
		}
		if !op.Changed {
			// Nothing was written: the issues of the file are the
			// entries of the list, and the author has to write them.
			continue
		}
		fixResult.ErrorsFixed += issueCounts[p]
		// The front matter changed: the fingerprint of the document
		// has to change with it.
		fingerprintTargets[p] = struct{}{}
	}
}

// WebSourcesUpdate records a web-sources fix.
type WebSourcesUpdate struct {
	FilePath string
	Changed  bool
	Success  bool
	Error    error
}

// normalizeWebSources rewrites a lone web_sources string as a list of
// one. A field that already is a list is left alone.
func (f *Fixer) normalizeWebSources(filePath string) WebSourcesUpdate {
	op := WebSourcesUpdate{FilePath: filePath, Success: true}

	// #nosec G304 -- filePath is derived from the current lint/fix target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for the web-sources fix: %w", err)
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

	raw, ok := fields["web_sources"]
	if !ok || raw == nil {
		return op
	}
	value, ok := raw.(string)
	if !ok {
		// The field already is a list: the issues of the file are the
		// entries of the list.
		return op
	}

	// The field is patched into the raw frontmatter: the comments of
	// the file and the order of its keys survive the fix (ADR-014).
	updated, patched := frontmatterops.PatchFrontmatter(data, body, had, style, map[string]any{
		"web_sources": []any{value},
	})
	if !patched {
		op.Success = false
		op.Error = fmt.Errorf("write frontmatter: the frontmatter of %s could not be patched", filePath)
		return op
	}

	if f.dryRun {
		op.Changed = true
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for the web-sources fix: %w", statErr)
		return op
	}

	if writeErr := foundation.WriteFileAtomic(filePath, updated, info.Mode().Perm()); writeErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("write file for the web-sources fix: %w", writeErr)
		return op
	}

	op.Changed = true
	return op
}
