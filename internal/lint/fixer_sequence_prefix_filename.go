package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// applySequencePrefixFilenameFixes renames files in sequence-numbered
// directories so the filename matches the required pattern:
//
//   <prefix>-NNN-slug.md (3-digit zero-padded)
//
// For example, `adr-foo.md` in `docs/adr/` becomes `adr-001-foo.md`.
// When there are multiple files in the same directory, the fixer
// numbers them in alphabetical order of the original filename to give
// a stable, deterministic sequence.
//
// Note: this fixer changes filenames, which in turn invalidates any
// cross-references from other docs. The existing rename logic in
// `processFileWithIssues` already handles link rewrites for filenames
// changed by `filename-conventions`; we re-use the same
// `RenameOperation` machinery by appending to `fixResult.FilesRenamed`
// and triggering the link-update pipeline.
//
// For simplicity and to avoid double-counting, this fixer runs
// directly via the existing `processFileWithIssues` invocation by
// tagging sequence-prefix-filename issues as filename-conventions;
// callers should treat sequence-prefix-filename as a sub-rule.
func (f *Fixer) applySequencePrefixFilenameFixes(targets map[string]struct{}, issueCounts map[string]int, fixResult *FixResult, rootPath string, fingerprintTargets map[string]struct{}) {
	if len(targets) == 0 {
		return
	}

	// Group files by directory so we can number them sequentially.
	byDir := make(map[string][]string)
	for p := range targets {
		dir := filepath.Dir(p)
		byDir[dir] = append(byDir[dir], p)
	}

	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		dirName := filepath.Base(dir)
		prefix, ok := sequencePrefixDirs[dirName]
		if !ok {
			// Skip directories not in the map (shouldn't happen since
			// the rule's AppliesTo gates this).
			continue
		}

		files := byDir[dir]
		sort.Strings(files)

		// Find the highest existing sequence number in this directory so
		// new entries don't collide.
		nextSeq := highestSequenceInDir(dir, prefix) + 1

		for _, p := range files {
			newName := buildSequenceName(prefix, nextSeq, filepath.Base(p))
			if newName == filepath.Base(p) {
				continue
			}
			newPath := filepath.Join(dir, newName)
			op := RenameOperation{
				OldPath: p,
				NewPath: newPath,
				Success: true,
			}
			if !f.dryRun {
				if err := os.Rename(p, newPath); err != nil {
					op.Success = false
					op.Error = fmt.Errorf("rename: %w", err)
					fixResult.Errors = append(fixResult.Errors, op.Error)
					nextSeq++
					continue
				}
			}
			fixResult.FilesRenamed = append(fixResult.FilesRenamed, op)
			fixResult.ErrorsFixed += issueCounts[p]
			// Renames invalidate content hashes.
			fingerprintTargets[newPath] = struct{}{}

			// Update links from the old path to the new path. The
			// broken-link detector would otherwise flag the renamed
			// file's old references as errors. Skipped in dry-run mode.
			if !f.dryRun {
				updates, err := f.findAndUpdateLinks(p, newPath, rootPath)
				if err != nil {
					fixResult.Errors = append(fixResult.Errors, err)
				} else {
					fixResult.LinksUpdated = append(fixResult.LinksUpdated, updates...)
					for _, upd := range updates {
						fingerprintTargets[upd.SourceFile] = struct{}{}
					}
				}
			}
			nextSeq++
		}
	}
}

// buildSequenceName constructs `<prefix>-NNN-<original-slug>.md`.
// If the original name already starts with a sequence number, that
// number is preserved and the prefix replaced; otherwise a fresh
// sequence is used.
func buildSequenceName(prefix string, seq int, original string) string {
	base := strings.TrimSuffix(original, ".md")
	slug := base
	if i := strings.Index(base, "-"); i >= 0 {
		// Strip an existing leading "<digits>-" prefix if present.
		head := base[:i]
		if isAllDigits(head) {
			if j := strings.Index(base[i+1:], "-"); j >= 0 {
				slug = base[i+j+2:]
			}
		}
	}
	if slug == "" {
		slug = base
	}
	return fmt.Sprintf("%s%03d-%s.md", prefix, seq, slug)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var seqFilePattern = regexp.MustCompile(`^([a-z]+)-(\d{3})-.+\.md$`)

// highestSequenceInDir scans dir for filenames matching the prefix
// pattern and returns the highest sequence number seen. Returns 0 if
// no matching files are found.
func highestSequenceInDir(dir, prefix string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	highest := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Match prefix- and extract the sequence number.
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		m := seqFilePattern.FindStringSubmatch(prefix + rest)
		if m == nil {
			continue
		}
		// m[1] is the prefix (may not match if user used a different
		// prefix in the filename), m[2] is the number.
		if m[1] != strings.TrimSuffix(prefix, "-") {
			continue
		}
		var n int
		fmt.Sscanf(m[2], "%d", &n)
		if n > highest {
			highest = n
		}
	}
	return highest
}

// sequencePrefixDirs mirrors the rule's static map. Kept as a package-
// level variable so callers can extend it without copying the rule.
var sequencePrefixDirs = map[string]string{
	"adr": "adr-",
}
