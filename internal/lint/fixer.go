package lint

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Fixer performs automatic fixes for linting issues.
type Fixer struct {
	linter   *Linter
	dryRun   bool
	force    bool
	gitAware bool
}

// NewFixer creates a new fixer with the given linter and options.
func NewFixer(linter *Linter, dryRun, force bool) *Fixer {
	return &Fixer{
		linter:   linter,
		dryRun:   dryRun,
		force:    force,
		gitAware: isGitRepository("."),
	}
}

// FixResult contains the results of a fix operation.
type FixResult struct {
	FilesRenamed  []RenameOperation
	LinksUpdated  []LinkUpdate
	ErrorsFixed   int
	WarningsFixed int
	Errors        []error
}

// RenameOperation represents a file rename operation.
type RenameOperation struct {
	OldPath string
	NewPath string
	Success bool
	Error   error
}

// LinkUpdate represents a link that was updated.
type LinkUpdate struct {
	SourceFile string
	LineNumber int
	OldTarget  string
	NewTarget  string
}

// LinkType represents the type of markdown link.
type LinkType int

const (
	LinkTypeInline LinkType = iota
	LinkTypeReference
	LinkTypeImage
)

// LinkReference represents a link found in a markdown file.
type LinkReference struct {
	SourceFile string   // File containing the link
	LineNumber int      // Line number of link
	LinkType   LinkType // Inline, Reference, or Image
	Target     string   // Link target (path)
	Fragment   string   // Anchor fragment (#section)
	FullMatch  string   // Complete original text for replacement
}

// Fix attempts to automatically fix issues found in the given path.
func (f *Fixer) Fix(path string) (*FixResult, error) {
	// First, run linter to find issues
	result, err := f.linter.LintPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to lint path: %w", err)
	}

	fixResult := &FixResult{
		FilesRenamed: make([]RenameOperation, 0),
		LinksUpdated: make([]LinkUpdate, 0),
		Errors:       make([]error, 0),
	}

	// Group issues by file
	fileIssues := make(map[string][]Issue)
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError {
			fileIssues[issue.FilePath] = append(fileIssues[issue.FilePath], issue)
		}
	}

	// Process each file with issues
	for filePath, issues := range fileIssues {
		// Check if this is a filename issue that can be fixed
		if f.canFixFilename(issues) {
			op := f.renameFile(filePath, issues)
			fixResult.FilesRenamed = append(fixResult.FilesRenamed, op)

			if op.Success {
				fixResult.ErrorsFixed += len(issues)

				// Find and update all links to the renamed file
				if !f.dryRun {
					links, err := f.findLinksToFile(filePath)
					if err != nil {
						fixResult.Errors = append(fixResult.Errors,
							fmt.Errorf("failed to find links to %s: %w", filePath, err))
					} else if len(links) > 0 {
						// Apply link updates
						updates, err := f.applyLinkUpdates(links, filePath, op.NewPath)
						if err != nil {
							fixResult.Errors = append(fixResult.Errors,
								fmt.Errorf("failed to update links: %w", err))
						} else {
							fixResult.LinksUpdated = append(fixResult.LinksUpdated, updates...)
						}
					}
				}
			} else if op.Error != nil {
				fixResult.Errors = append(fixResult.Errors, op.Error)
			}
		}
	}

	return fixResult, nil
}

// canFixFilename checks if the issues for a file are filename-related and fixable.
func (f *Fixer) canFixFilename(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Rule == "filename-conventions" {
			return true
		}
	}
	return false
}

// renameFile renames a file to fix filename issues.
func (f *Fixer) renameFile(oldPath string, issues []Issue) RenameOperation {
	op := RenameOperation{
		OldPath: oldPath,
		Success: false,
	}

	// Get the suggested filename using the same logic as the linter
	filename := filepath.Base(oldPath)
	suggestedName := SuggestFilename(filename)

	if suggestedName == "" || suggestedName == filename {
		op.Error = fmt.Errorf("could not determine suggested filename or file is already correct")
		return op
	}

	// Construct new path
	dir := filepath.Dir(oldPath)
	newPath := filepath.Join(dir, suggestedName)
	op.NewPath = newPath

	// Check if target already exists
	if _, err := os.Stat(newPath); err == nil && !f.force {
		op.Error = fmt.Errorf("target file already exists: %s", newPath)
		return op
	}

	// Dry-run mode: just report what would happen
	if f.dryRun {
		op.Success = true
		return op
	}

	// Perform the rename
	if f.gitAware && f.shouldUseGitMv(oldPath) {
		// Use git mv to preserve history
		err := f.gitMv(oldPath, newPath)
		if err != nil {
			op.Error = fmt.Errorf("git mv failed: %w", err)
			return op
		}
	} else {
		// Use regular file system rename
		err := os.Rename(oldPath, newPath)
		if err != nil {
			op.Error = fmt.Errorf("rename failed: %w", err)
			return op
		}
	}

	op.Success = true
	return op
}

// shouldUseGitMv checks if a file is under Git version control.
func (f *Fixer) shouldUseGitMv(filePath string) bool {
	if !f.gitAware {
		return false
	}

	// Check if file is tracked by Git
	cmd := exec.Command("git", "ls-files", "--error-unmatch", filePath)
	err := cmd.Run()
	return err == nil
}

// gitMv performs a git mv operation.
func (f *Fixer) gitMv(oldPath, newPath string) error {
	cmd := exec.Command("git", "mv", oldPath, newPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// isGitRepository checks if the given directory is a Git repository.
func isGitRepository(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// HasErrors returns true if any errors occurred during fixing.
func (fr *FixResult) HasErrors() bool {
	return len(fr.Errors) > 0
}

// findLinksToFile finds all markdown links that reference the given target file.
func (f *Fixer) findLinksToFile(targetPath string) ([]LinkReference, error) {
	var links []LinkReference

	// Get absolute path of target for comparison
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for target: %w", err)
	}

	// Walk the directory containing the target file
	rootDir := filepath.Dir(targetPath)
	if info, err := os.Stat(targetPath); err == nil && info.IsDir() {
		rootDir = targetPath
	}

	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process markdown files
		if info.IsDir() || !IsDocFile(path) {
			return nil
		}

		// Don't search the target file itself
		if path == targetPath {
			return nil
		}

		// Find links in this file
		fileLinks, err := f.findLinksInFile(path, absTarget)
		if err != nil {
			return fmt.Errorf("failed to scan %s: %w", path, err)
		}

		links = append(links, fileLinks...)
		return nil
	})

	return links, err
}

// findLinksInFile scans a single markdown file for links to the target.
func (f *Fixer) findLinksInFile(sourceFile, targetPath string) ([]LinkReference, error) {
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var links []LinkReference
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		// Skip code blocks (simple heuristic: lines starting with spaces/tabs or in fenced blocks)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			continue
		}

		// Find inline links: [text](path)
		inlineLinks := findInlineLinks(line, lineNum+1, sourceFile, targetPath)
		links = append(links, inlineLinks...)

		// Find reference-style links: [id]: path
		refLinks := findReferenceLinks(line, lineNum+1, sourceFile, targetPath)
		links = append(links, refLinks...)

		// Find image links: ![alt](path)
		imageLinks := findImageLinks(line, lineNum+1, sourceFile, targetPath)
		links = append(links, imageLinks...)
	}

	return links, nil
}

// resolveRelativePath resolves a relative link path from a source file to an absolute path.
func resolveRelativePath(sourceFile, linkTarget string) (string, error) {
	// Remove any URL fragments (#section)
	targetPath := strings.Split(linkTarget, "#")[0]

	// Get directory of source file
	sourceDir := filepath.Dir(sourceFile)

	// Join and clean the path
	resolvedPath := filepath.Join(sourceDir, targetPath)
	cleanPath := filepath.Clean(resolvedPath)

	// Get absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return absPath, nil
}

// findInlineLinks finds inline-style markdown links: [text](path)
func findInlineLinks(line string, lineNum int, sourceFile, targetPath string) []LinkReference {
	// Pattern: [text](path) or [text](path#fragment)
	// We'll use a simple string search for now, can be improved with regex
	var links []LinkReference

	// Look for ]( pattern
	for i := 0; i < len(line); i++ {
		if i+1 < len(line) && line[i] == ']' && line[i+1] == '(' {
			// Find the opening [
			start := -1
			for j := i - 1; j >= 0; j-- {
				if line[j] == '[' {
					// Make sure it's not an image link (preceded by !)
					if j > 0 && line[j-1] == '!' {
						break
					}
					start = j
					break
				}
			}

			if start == -1 {
				continue
			}

			// Find the closing )
			end := strings.Index(line[i+2:], ")")
			if end == -1 {
				continue
			}
			end += i + 2

			// Extract the link target
			linkTarget := line[i+2 : end]

			// Skip external URLs
			if strings.HasPrefix(linkTarget, "http://") || strings.HasPrefix(linkTarget, "https://") {
				continue
			}

			// Resolve the path
			resolved, err := resolveRelativePath(sourceFile, linkTarget)
			if err != nil {
				continue
			}

			// Check if this link points to our target
			if resolved == targetPath {
				// Extract fragment if present
				fragment := ""
				if idx := strings.Index(linkTarget, "#"); idx != -1 {
					fragment = linkTarget[idx:]
					linkTarget = linkTarget[:idx]
				}

				links = append(links, LinkReference{
					SourceFile: sourceFile,
					LineNumber: lineNum,
					LinkType:   LinkTypeInline,
					Target:     linkTarget,
					Fragment:   fragment,
					FullMatch:  line[start : end+1],
				})
			}
		}
	}

	return links
}

// findReferenceLinks finds reference-style markdown links: [id]: path
func findReferenceLinks(line string, lineNum int, sourceFile, targetPath string) []LinkReference {
	var links []LinkReference

	// Pattern: [id]: path or [id]: path "title"
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return links
	}

	// Find closing ]
	endBracket := strings.Index(trimmed, "]:")
	if endBracket == -1 {
		return links
	}

	// Extract the path part (after ]:)
	rest := strings.TrimSpace(trimmed[endBracket+2:])
	if rest == "" {
		return links
	}

	// Remove optional title in quotes
	linkTarget := rest
	if idx := strings.Index(rest, " \""); idx != -1 {
		linkTarget = rest[:idx]
	} else if idx := strings.Index(rest, " '"); idx != -1 {
		linkTarget = rest[:idx]
	}

	linkTarget = strings.TrimSpace(linkTarget)

	// Skip external URLs
	if strings.HasPrefix(linkTarget, "http://") || strings.HasPrefix(linkTarget, "https://") {
		return links
	}

	// Resolve the path
	resolved, err := resolveRelativePath(sourceFile, linkTarget)
	if err != nil {
		return links
	}

	// Check if this link points to our target
	if resolved == targetPath {
		// Extract fragment if present
		fragment := ""
		if idx := strings.Index(linkTarget, "#"); idx != -1 {
			fragment = linkTarget[idx:]
			linkTarget = linkTarget[:idx]
		}

		links = append(links, LinkReference{
			SourceFile: sourceFile,
			LineNumber: lineNum,
			LinkType:   LinkTypeReference,
			Target:     linkTarget,
			Fragment:   fragment,
			FullMatch:  line,
		})
	}

	return links
}

// findImageLinks finds image markdown links: ![alt](path)
func findImageLinks(line string, lineNum int, sourceFile, targetPath string) []LinkReference {
	var links []LinkReference

	// Look for ![]( pattern
	for i := 0; i < len(line); i++ {
		if i+2 < len(line) && line[i] == '!' && line[i+1] == '[' {
			// Find the closing ]
			closeBracket := strings.Index(line[i+2:], "]")
			if closeBracket == -1 {
				continue
			}
			closeBracket += i + 2

			// Check for opening (
			if closeBracket+1 >= len(line) || line[closeBracket+1] != '(' {
				continue
			}

			// Find the closing )
			end := strings.Index(line[closeBracket+2:], ")")
			if end == -1 {
				continue
			}
			end += closeBracket + 2

			// Extract the link target
			linkTarget := line[closeBracket+2 : end]

			// Skip external URLs
			if strings.HasPrefix(linkTarget, "http://") || strings.HasPrefix(linkTarget, "https://") {
				continue
			}

			// Resolve the path
			resolved, err := resolveRelativePath(sourceFile, linkTarget)
			if err != nil {
				continue
			}

			// Check if this link points to our target
			if resolved == targetPath {
				links = append(links, LinkReference{
					SourceFile: sourceFile,
					LineNumber: lineNum,
					LinkType:   LinkTypeImage,
					Target:     linkTarget,
					Fragment:   "",
					FullMatch:  line[i : end+1],
				})
			}
		}
	}

	return links
}

// applyLinkUpdates applies link updates to markdown files atomically.
// All updates succeed or all fail (atomic transaction with rollback).
func (f *Fixer) applyLinkUpdates(links []LinkReference, oldPath, newPath string) ([]LinkUpdate, error) {
	// Group links by source file
	fileLinks := make(map[string][]LinkReference)
	for _, link := range links {
		fileLinks[link.SourceFile] = append(fileLinks[link.SourceFile], link)
	}

	var updates []LinkUpdate
	var updatedFiles []string
	var backupPaths []string

	// Process each file
	for sourceFile, linkRefs := range fileLinks {
		// Read file content
		content, err := os.ReadFile(sourceFile)
		if err != nil {
			// Rollback any previous changes
			f.rollbackLinkUpdates(updatedFiles, backupPaths)
			return nil, fmt.Errorf("failed to read %s: %w", sourceFile, err)
		}

		lines := strings.Split(string(content), "\n")
		modified := false

		// Sort links by line number in reverse order to maintain line offsets
		// (updating from bottom to top preserves line numbers)
		sortedLinks := make([]LinkReference, len(linkRefs))
		copy(sortedLinks, linkRefs)
		for i := 0; i < len(sortedLinks); i++ {
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
			err := os.WriteFile(backupPath, content, 0644)
			if err != nil {
				// Rollback previous changes
				f.rollbackLinkUpdates(updatedFiles, backupPaths)
				return nil, fmt.Errorf("failed to create backup for %s: %w", sourceFile, err)
			}
			backupPaths = append(backupPaths, backupPath)

			// Write updated content
			err = os.WriteFile(sourceFile, []byte(newContent), 0644)
			if err != nil {
				// Rollback previous changes
				f.rollbackLinkUpdates(updatedFiles, backupPaths)
				return nil, fmt.Errorf("failed to write updated %s: %w", sourceFile, err)
			}

			updatedFiles = append(updatedFiles, sourceFile)
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
// - Link style (relative vs absolute within repo)
func (f *Fixer) updateLinkTarget(link LinkReference, oldPath, newPath string) string {
	// Get the new filename
	newFilename := filepath.Base(newPath)

	// Preserve relative path structure
	oldFilename := filepath.Base(oldPath)

	// Replace only the filename portion, keeping the directory path
	newTarget := strings.Replace(link.Target, oldFilename, newFilename, 1)

	// Preserve anchor fragment if present
	if link.Fragment != "" {
		newTarget = newTarget + link.Fragment
	}

	return newTarget
}

// rollbackLinkUpdates restores files from their backups.
// This provides transactional rollback on any failure during link updates.
func (f *Fixer) rollbackLinkUpdates(files []string, backupPaths []string) {
	for _, backupPath := range backupPaths {
		// Extract original file path by removing .backup suffix
		originalFile := strings.TrimSuffix(backupPath, ".backup")

		if content, err := os.ReadFile(backupPath); err == nil {
			_ = os.WriteFile(originalFile, content, 0644) // Best effort restore
		}
		_ = os.Remove(backupPath) // Best effort cleanup
	}
}

// Summary returns a human-readable summary of the fix operation.
func (fr *FixResult) Summary() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Files renamed: %d\n", len(fr.FilesRenamed)))
	b.WriteString(fmt.Sprintf("Errors fixed: %d\n", fr.ErrorsFixed))
	b.WriteString(fmt.Sprintf("Links updated: %d\n", len(fr.LinksUpdated)))

	if len(fr.LinksUpdated) > 0 {
		b.WriteString("\nLink Updates:\n")
		for _, update := range fr.LinksUpdated {
			b.WriteString(fmt.Sprintf("  • %s:%d: %s → %s\n",
				update.SourceFile, update.LineNumber, update.OldTarget, update.NewTarget))
		}
	}

	if len(fr.Errors) > 0 {
		b.WriteString(fmt.Sprintf("\nErrors encountered: %d\n", len(fr.Errors)))
		for _, err := range fr.Errors {
			b.WriteString(fmt.Sprintf("  • %v\n", err))
		}
	}

	return b.String()
}
