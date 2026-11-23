package lint

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/markdown"
)

// applyLinkUpdates applies link updates to markdown files atomically.
// All updates succeed or all fail (atomic transaction with rollback).
func (f *Fixer) applyLinkUpdates(links []LinkReference, oldPath, newPath string) ([]LinkUpdate, error) {
	// Group links by source file
	fileLinks := make(map[string][]LinkReference)
	for _, link := range links {
		fileLinks[link.SourceFile] = append(fileLinks[link.SourceFile], link)
	}

	var updates []LinkUpdate
	var backupPaths []string

	// Process each file
	for sourceFile, linkRefs := range fileLinks {
		// Read file content
		// #nosec G304 -- sourceFile is from link discovery, validated paths
		content, err := os.ReadFile(sourceFile)
		if err != nil {
			// Rollback any previous changes
			f.rollbackLinkUpdates(backupPaths)
			return nil, fmt.Errorf("failed to read %s: %w", sourceFile, err)
		}

		originalContent := append([]byte(nil), content...)
		modified := false

		// Sort links by line number in reverse order to maintain line offsets
		// (updating from bottom to top preserves line numbers)
		sortedLinks := slices.Clone(linkRefs)
		slices.SortStableFunc(sortedLinks, func(a, b LinkReference) int {
			return b.LineNumber - a.LineNumber
		})

		// Apply updates to each link
		for _, link := range sortedLinks {
			lineStart, lineEnd, ok := findLineByteRange(content, link.LineNumber)
			if !ok {
				continue
			}

			// Generate new link target
			newTarget := f.updateLinkTarget(link, oldPath, newPath)

			// For comparison, combine target with fragment for old link
			oldLinkText := link.Target + link.Fragment

			if newTarget == oldLinkText {
				continue // No change needed
			}

			line := content[lineStart:lineEnd]
			// The target is rewritten where it is the target of a link.
			// A file that is mentioned in the prose of the same line is
			// not the target of the link, and rewriting it would change
			// the words of the page.
			idx := linkTargetIndex(line, oldLinkText)
			if idx == -1 {
				continue
			}

			updated, err := markdown.ApplyEdits(content, []markdown.Edit{{
				Start:       lineStart + idx,
				End:         lineStart + idx + len(oldLinkText),
				Replacement: []byte(newTarget),
			}})
			if err != nil {
				f.rollbackLinkUpdates(backupPaths)
				return nil, fmt.Errorf("failed to apply link updates to %s: %w", sourceFile, err)
			}

			content = updated
			modified = true

			updates = append(updates, LinkUpdate{
				SourceFile: sourceFile,
				LineNumber: link.LineNumber,
				OldTarget:  oldLinkText,
				NewTarget:  newTarget,
			})
		}

		// Write updated content if modified
		if modified {
			// Create backup before writing
			backupPath := sourceFile + ".backup"
			//nolint:gosec // the backup sits next to the file it copies
			err := os.WriteFile(backupPath, originalContent, 0o600)
			if err != nil {
				// Rollback previous changes
				f.rollbackLinkUpdates(backupPaths)
				return nil, fmt.Errorf("failed to create backup for %s: %w", sourceFile, err)
			}
			backupPaths = append(backupPaths, backupPath)

			// Write updated content
			err = foundation.WriteFileAtomic(sourceFile, content, 0o600)
			if err != nil {
				// Rollback previous changes
				f.rollbackLinkUpdates(backupPaths)
				return nil, fmt.Errorf("failed to write updated %s: %w", sourceFile, err)
			}

			// The file changed, so its links have to be read again.
			delete(f.linkCache, sourceFile)
		}
	}

	// All updates succeeded - clean up backup files
	for _, backupPath := range backupPaths {
		_ = os.Remove(backupPath) // Ignore cleanup errors
	}

	return updates, nil
}

// linkTargetIndex returns the offset of the target of a link on the
// line, or -1 when the line holds no link to it.
//
// The target is looked for inside the link: "](target)" and
// "](<target>)" for an inline link, "]: target" for a reference. The
// last link on the line is the one that is rewritten, so a label that
// holds the same text is not touched.
func linkTargetIndex(line []byte, target string) int {
	if idx := bytes.LastIndex(line, []byte("]("+target)); idx != -1 {
		return idx + 2
	}
	if idx := bytes.LastIndex(line, []byte("](<"+target+">)")); idx != -1 {
		return idx + 3
	}
	if idx := bytes.LastIndex(line, []byte("]: "+target)); idx != -1 {
		return idx + 3
	}
	return -1
}

func findLineByteRange(content []byte, lineNumber int) (int, int, bool) {
	if lineNumber <= 0 {
		return 0, 0, false
	}

	start := 0
	current := 1
	for current < lineNumber {
		idx := bytes.IndexByte(content[start:], '\n')
		if idx == -1 {
			return 0, 0, false
		}
		start = start + idx + 1
		current++
	}

	endRel := bytes.IndexByte(content[start:], '\n')
	if endRel == -1 {
		return start, len(content), true
	}
	return start, start + endRel, true
}

// updateLinkTarget generates a new link target for a renamed file.
// It preserves:
// - Relative path structure (./path, ../path, path)
// - Anchor fragments (#section)
// - Link style (relative vs absolute within repo).
func (f *Fixer) updateLinkTarget(link LinkReference, oldPath, newPath string) string {
	originalTarget := link.Target + link.Fragment
	resolved, resErr := resolveRelativePath(link.SourceFile, originalTarget)
	// The path of the walk is relative and the resolved link is
	// absolute: without the same shape the precise rewrite is skipped
	// and the link keeps the old name.
	absoluteOld, absErr := filepath.Abs(oldPath)
	if resErr == nil && absErr == nil && pathsEqualCaseInsensitive(resolved, absoluteOld) {
		updated, changed, err := computeUpdatedLinkTarget(link.SourceFile, originalTarget, oldPath, newPath)
		if err == nil && changed {
			return updated
		}
	}

	// Fallback: keep legacy behavior (filename-only replacement).
	newFilename := filepath.Base(newPath)
	oldFilename := filepath.Base(oldPath)
	newTarget := strings.Replace(link.Target, oldFilename, newFilename, 1)
	if link.Fragment != "" {
		newTarget += link.Fragment
	}
	return newTarget
}

// rollbackLinkUpdates restores files from their backups.
// This provides transactional rollback on any failure during link updates.
func (f *Fixer) rollbackLinkUpdates(backupPaths []string) {
	for _, backupPath := range backupPaths {
		// Extract original file path by removing .backup suffix
		originalFile := strings.TrimSuffix(backupPath, ".backup")

		// #nosec G304 -- backupPath created internally, not user input
		if content, err := os.ReadFile(backupPath); err == nil {
			//nolint:gosec // the file is the one the link update changed
			_ = os.WriteFile(originalFile, content, 0o600)
		}
		_ = os.Remove(backupPath) // Best effort cleanup
	}
}
