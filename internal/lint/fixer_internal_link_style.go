package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/docmodel"
)

// applyInternalLinkStyleFixes rewrites internal markdown links to
// include the `.md` extension. The fixer is conservative:
//
//   - only relative targets that don't already end in .md / .markdown
//     are rewritten (not external URLs, not site-rooted, not anchors)
//   - the rewrite is a pure extension append: `../foo` → `../foo.md`
//   - existing anchors (`#section`) and query strings (`?x=1`) are
//     preserved at the end of the target
//   - inline code and fenced/indented code blocks are skipped
//
// Site-rooted links (starting with `/`) are NOT auto-fixed — those
// need user judgment.
func (f *Fixer) applyInternalLinkStyleFixes(targets map[string]struct{}, issueCounts map[string]int, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
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

		op := f.appendMarkdownExtension(p)
		if op.Success {
			fixResult.LinkStyleUpdates = append(fixResult.LinkStyleUpdates, op)
			fixResult.ErrorsFixed += issueCounts[p]
			fingerprintTargets[p] = struct{}{}
			continue
		}
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
		}
	}
}

// LinkStyleUpdate records a link-style fix on a file.
type LinkStyleUpdate struct {
	FilePath string
	Updated  int // number of link targets rewritten
	Success  bool
	Error    error
}

// appendMarkdownExtension scans the body for inline links whose
// targets are relative paths without `.md`, and rewrites them.
// Uses docmodel.ParseFile so code blocks and inline code are skipped
// (matching the existing broken-link detector).
func (f *Fixer) appendMarkdownExtension(filePath string) LinkStyleUpdate {
	op := LinkStyleUpdate{FilePath: filePath, Success: true}

	// #nosec G304 -- filePath is derived from the current lint/fix target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for link-style fix: %w", err)
		return op
	}

	doc, err := docmodel.Parse(data, docmodel.Options{})
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("parse doc for link-style fix: %w", err)
		return op
	}

	refs, err := doc.LinkRefs()
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("extract link refs: %w", err)
		return op
	}

	type rewrite struct {
		old string
		new string
	}
	var rewrites []rewrite
	updated := 0
	for _, ref := range refs {
		if ref.Link.Kind != "inline" {
			continue
		}
		target := ref.Link.Destination
		if target == "" || isExternalLinkTarget(target) || isAnchorLinkTarget(target) || isSiteRootedLinkTarget(target) {
			continue
		}
		// Strip query/fragment for the extension check.
		base := target
		var suffix string
		if i := strings.IndexAny(base, "?#"); i >= 0 {
			suffix = base[i:]
			base = base[:i]
		}
		// Skip if:
		//  - already has a markdown extension (no work needed)
		//  - has any other file extension (don't clobber .go, .png, etc.)
		//  - ends with `/` (target is a directory, not a file)
		if hasMarkdownExt(base) || filepath.Ext(base) != "" || strings.HasSuffix(base, "/") {
			continue
		}
		newTarget := base + ".md" + suffix
		if newTarget == target {
			continue
		}
		rewrites = append(rewrites, rewrite{old: target, new: newTarget})
		updated++
	}

	if updated == 0 {
		return op
	}

	// Apply rewrites by replacing each original target substring with
	// its `.md`-suffixed form. We anchor on `](` so the rewrite stays
	// scoped to the link syntax. We splice in the new target then keep
	// the closing `)` of the link.
	newData := string(data)
	for _, rw := range rewrites {
		anchorOpen := "](" + rw.old + ")"
		idx := strings.Index(newData, anchorOpen)
		if idx < 0 {
			continue
		}
		// newData[:idx+2] is everything up to and including `(`.
		// We then insert rw.new (the new target), then keep the `)`
		// from the original (it's the last char of anchorOpen).
		newData = newData[:idx+2] + rw.new + ")" + newData[idx+len(anchorOpen):]
	}

	if newData == string(data) {
		return op
	}

	if f.dryRun {
		op.Updated = updated
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for link-style fix: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, []byte(newData), info.Mode().Perm()); writeErr != nil { //nolint:gosec // filePath is derived from the lint target set
		op.Success = false
		op.Error = fmt.Errorf("write file for link-style fix: %w", writeErr)
		return op
	}

	op.Updated = updated
	return op
}

func isExternalLinkTarget(t string) bool {
	return strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://")
}

func isAnchorLinkTarget(t string) bool {
	return strings.HasPrefix(t, "#")
}

func isSiteRootedLinkTarget(t string) bool {
	return strings.HasPrefix(t, "/")
}

func hasMarkdownExt(t string) bool {
	return strings.HasSuffix(t, ".md") || strings.HasSuffix(t, ".markdown")
}
