package lint

import (
	"os"
	"path/filepath"
	"strings"

	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
			return nil, foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
				"failed to read file for link update").
				WithContext("file", sourceFile).
				Build()
		}

		lines := strings.Split(string(content), "\n")
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
			lineIdx := link.LineNumber - 1
			if lineIdx < 0 || lineIdx >= len(lines) {
				continue
			}

			// Generate new link target
			newTarget := f.updateLinkTarget(link, oldPath, newPath)

			// For comparison, combine target with fragment for old link
			oldLinkText := link.Target + link.Fragment

			if newTarget == oldLinkText {
				continue // No change needed
			}

			// Replace the old link text with the new target in the line
			oldLine := lines[lineIdx]
			newLine := strings.Replace(oldLine, oldLinkText, newTarget, 1)

			if newLine != oldLine {
				lines[lineIdx] = newLine
				modified = true

				updates = append(updates, LinkUpdate{
					SourceFile: sourceFile,
					LineNumber: link.LineNumber,
					OldTarget:  oldLinkText,
					NewTarget:  newTarget,
				})
			}
		}

		// Write updated content if modified
		if modified {
			newContent := strings.Join(lines, "\n")

			// Create backup before writing
			backupPath := sourceFile + ".backup"
			err := os.WriteFile(backupPath, content, 0o600)
			if err != nil {
				// Rollback previous changes
				f.rollbackLinkUpdates(backupPaths)
				return nil, foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
					"failed to create backup for link update").
					WithContext("file", sourceFile).
					WithContext("backup_path", backupPath).
					Build()
			}
			backupPaths = append(backupPaths, backupPath)

			// Write updated content
			err = os.WriteFile(sourceFile, []byte(newContent), 0o600)
			if err != nil {
				// Rollback previous changes
				f.rollbackLinkUpdates(backupPaths)
				return nil, foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
					"failed to write updated file with link changes").
					WithContext("file", sourceFile).
					Build()
			}
		}
	}

	// All updates succeeded - clean up backup files
	for _, backupPath := range backupPaths {
		_ = os.Remove(backupPath) // Ignore cleanup errors
	}

	return updates, nil
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
