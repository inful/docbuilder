package lint

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		sortedLinks := make([]LinkReference, len(linkRefs))
		copy(sortedLinks, linkRefs)
		for i := range sortedLinks {
			for j := i + 1; j < len(sortedLinks); j++ {
				if sortedLinks[i].LineNumber < sortedLinks[j].LineNumber {
					sortedLinks[i], sortedLinks[j] = sortedLinks[j], sortedLinks[i]
				}
			}
		}

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
			idx := bytes.Index(line, []byte(oldLinkText))
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
			err := os.WriteFile(backupPath, originalContent, 0o600)
			if err != nil {
				// Rollback previous changes
				f.rollbackLinkUpdates(backupPaths)
				return nil, fmt.Errorf("failed to create backup for %s: %w", sourceFile, err)
			}
			backupPaths = append(backupPaths, backupPath)

			// Write updated content
			err = os.WriteFile(sourceFile, content, 0o600)
			if err != nil {
				// Rollback previous changes
				f.rollbackLinkUpdates(backupPaths)
				return nil, fmt.Errorf("failed to write updated %s: %w", sourceFile, err)
			}
		}
	}

	// All updates succeeded - clean up backup files
	for _, backupPath := range backupPaths {
		_ = os.Remove(backupPath) // Ignore cleanup errors
	}

	return updates, nil
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
	// Get the new filename
	newFilename := filepath.Base(newPath)

	// Preserve relative path structure
	oldFilename := filepath.Base(oldPath)

	// Replace only the filename portion, keeping the directory path
	newTarget := strings.Replace(link.Target, oldFilename, newFilename, 1)

	// Preserve anchor fragment if present
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
			_ = os.WriteFile(originalFile, content, 0o600) // Best effort restore
		}
		_ = os.Remove(backupPath) // Best effort cleanup
	}
}
