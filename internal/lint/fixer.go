package lint

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Fixer performs automatic fixes for linting issues.
type Fixer struct {
	linter      *Linter
	dryRun      bool
	force       bool
	gitAware    bool
	autoConfirm bool // Skip confirmation prompts (for CI/automated use)
}

// NewFixer creates a new fixer with the given linter and options.
func NewFixer(linter *Linter, dryRun, force bool) *Fixer {
	return &Fixer{
		linter:      linter,
		dryRun:      dryRun,
		force:       force,
		gitAware:    isGitRepository("."),
		autoConfirm: false,
	}
}

// BrokenLink represents a link to a non-existent file.
type BrokenLink struct {
	SourceFile string // File containing the broken link
	LineNumber int    // Line number of the link
	Target     string // Link target that doesn't exist
	LinkType   LinkType
}

// FixResult contains the results of a fix operation.
type FixResult struct {
	FilesRenamed  []RenameOperation
	LinksUpdated  []LinkUpdate
	BrokenLinks   []BrokenLink // Links to non-existent files
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
// For interactive use with confirmation prompts, use FixWithConfirmation instead.
func (f *Fixer) Fix(path string) (*FixResult, error) {
	return f.fix(path)
}

// FixWithConfirmation fixes issues with interactive confirmation prompts.
// Shows a preview of changes and prompts the user before applying.
// Creates a backup of modified files before making changes.
//
//nolint:forbidigo // fmt is used for user-facing messages
func (f *Fixer) FixWithConfirmation(path string) (*FixResult, error) {
	// If in dry-run mode, no need for confirmation or backup
	if f.dryRun {
		return f.fix(path)
	}

	// Phase 1: Preview changes (dry-run mode internally)
	originalDryRun := f.dryRun
	f.dryRun = true
	previewResult, err := f.fix(path)
	f.dryRun = originalDryRun

	if err != nil {
		return nil, err
	}

	// If no changes to make, return early
	if !previewResult.HasChanges() {
		return previewResult, nil
	}

	// Phase 2: Show preview and get confirmation (unless auto-confirm)
	confirmed, err := f.ConfirmChanges(previewResult)
	if err != nil {
		return nil, fmt.Errorf("confirmation failed: %w", err)
	}

	if !confirmed {
		return previewResult, errors.New("user canceled operation")
	}

	// Phase 3: Create backup (always, even with auto-confirm)
	rootPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	backupDir, err := f.CreateBackup(previewResult, rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	if backupDir != "" {
		fmt.Printf("Created backup: %s\n", backupDir)
	}

	// Phase 4: Apply fixes
	return f.fix(path)
}

// fix is the internal implementation that actually performs the fixes.
func (f *Fixer) fix(path string) (*FixResult, error) {
	// First, run linter to find issues
	result, err := f.linter.LintPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to lint path: %w", err)
	}

	fixResult := &FixResult{
		FilesRenamed: make([]RenameOperation, 0),
		LinksUpdated: make([]LinkUpdate, 0),
		BrokenLinks:  make([]BrokenLink, 0),
		Errors:       make([]error, 0),
	}

	// Get absolute path for the root directory (for searching links)
	rootPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Detect broken links before applying fixes
	brokenLinks, err := detectBrokenLinks(path)
	if err != nil {
		// Non-fatal: log but continue with fixes
		fixResult.Errors = append(fixResult.Errors,
			fmt.Errorf("failed to detect broken links: %w", err))
	} else {
		fixResult.BrokenLinks = brokenLinks
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
			op := f.renameFile(filePath)
			fixResult.FilesRenamed = append(fixResult.FilesRenamed, op)

			if op.Success {
				fixResult.ErrorsFixed += len(issues)

				// Find and update all links to the renamed file
				if !f.dryRun {
					links, err := f.findLinksToFile(filePath, rootPath)
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

// WithAutoConfirm sets the auto-confirm flag for non-interactive mode.
// When true, skips confirmation prompts (useful for CI/automated workflows).
func (f *Fixer) WithAutoConfirm(autoConfirm bool) *Fixer {
	f.autoConfirm = autoConfirm
	return f
}

// ConfirmChanges prompts the user to confirm the proposed changes.
// Returns true if user confirms, false if they decline or if there's an error.
// Automatically returns true if autoConfirm is enabled or in dry-run mode.
//
//nolint:forbidigo // fmt is used for user-facing messages
func (f *Fixer) ConfirmChanges(result *FixResult) (bool, error) {
	// Auto-confirm in dry-run mode or if autoConfirm flag is set
	if f.dryRun || f.autoConfirm {
		return true, nil
	}

	// Nothing to confirm if no changes
	if !result.HasChanges() {
		return true, nil
	}

	// Show preview
	fmt.Println(result.PreviewChanges())

	// Prompt for confirmation
	fmt.Printf("Proceed with these changes? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// CreateBackup creates a backup of files that will be modified.
// Returns the backup directory path and any error encountered.
func (f *Fixer) CreateBackup(result *FixResult, rootPath string) (string, error) {
	// Skip backup in dry-run mode
	if f.dryRun {
		return "", nil
	}

	// Create backup directory with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(rootPath, fmt.Sprintf(".docbuilder-backup-%s", timestamp))

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Backup files that will be renamed
	for _, rename := range result.FilesRenamed {
		if err := f.backupFile(rename.OldPath, backupDir, rootPath); err != nil {
			return "", fmt.Errorf("failed to backup %s: %w", rename.OldPath, err)
		}
	}

	// Backup files that will have links updated
	backedUp := make(map[string]bool)
	for _, update := range result.LinksUpdated {
		// Avoid backing up the same file multiple times
		if backedUp[update.SourceFile] {
			continue
		}
		if err := f.backupFile(update.SourceFile, backupDir, rootPath); err != nil {
			return "", fmt.Errorf("failed to backup %s: %w", update.SourceFile, err)
		}
		backedUp[update.SourceFile] = true
	}

	return backupDir, nil
}

// backupFile copies a file to the backup directory, preserving directory structure.
func (f *Fixer) backupFile(filePath, backupDir, rootPath string) error {
	// Get relative path from root
	relPath, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		// If relative path fails, use just the filename
		relPath = filepath.Base(filePath)
	}

	// Create destination path in backup directory
	backupPath := filepath.Join(backupDir, relPath)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return err
	}

	// Copy file
	return f.copyFile(filePath, backupPath)
}

// copyFile copies a file from src to dst.
func (f *Fixer) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// renameFile renames a file to fix filename issues.
func (f *Fixer) renameFile(oldPath string) RenameOperation {
	op := RenameOperation{
		OldPath: oldPath,
		Success: false,
	}

	// Get the suggested filename using the same logic as the linter
	filename := filepath.Base(oldPath)
	suggestedName := SuggestFilename(filename)

	if suggestedName == "" || suggestedName == filename {
		op.Error = errors.New("could not determine suggested filename or file is already correct")
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
	return len(fr.Errors) > 0 || len(fr.BrokenLinks) > 0
}

// findLinksToFile finds all markdown links that reference the given target file.
// It searches from rootPath (typically the documentation root directory) to find
// all markdown files that might contain links to the target.
func (f *Fixer) findLinksToFile(targetPath, rootPath string) ([]LinkReference, error) {
	var links []LinkReference

	// Get absolute path of target for comparison
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for target: %w", err)
	}

	// Ensure rootPath is a directory
	rootInfo, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat root path: %w", err)
	}

	searchRoot := rootPath
	if !rootInfo.IsDir() {
		searchRoot = filepath.Dir(rootPath)
	}

	err = filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
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

// detectBrokenLinks scans all markdown files in a path for links to non-existent files.
func detectBrokenLinks(rootPath string) ([]BrokenLink, error) {
	var brokenLinks []BrokenLink

	// Determine if rootPath is a file or directory
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	var filesToScan []string
	if info.IsDir() {
		// Walk directory to find all markdown files
		err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip hidden directories and files
			if info.Name() != "." && strings.HasPrefix(info.Name(), ".") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if info.IsDir() {
				return nil
			}

			// Skip standard ignored files (case-insensitive)
			if isIgnoredFile(info.Name()) {
				return nil
			}

			if IsDocFile(path) {
				filesToScan = append(filesToScan, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else if IsDocFile(rootPath) {
		filesToScan = []string{rootPath}
	}

	// Scan each file for broken links
	for _, file := range filesToScan {
		broken, err := detectBrokenLinksInFile(file)
		if err != nil {
			// Continue with other files even if one fails
			continue
		}
		brokenLinks = append(brokenLinks, broken...)
	}

	return brokenLinks, nil
}

// detectBrokenLinksInFile scans a single markdown file for broken links.
func detectBrokenLinksInFile(sourceFile string) ([]BrokenLink, error) {
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var brokenLinks []BrokenLink
	lines := strings.Split(string(content), "\n")

	inCodeBlock := false
	for lineNum, line := range lines {
		// Track code block boundaries
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		// Skip lines inside code blocks or indented code blocks
		if inCodeBlock || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			continue
		}

		// Check inline links
		broken := checkInlineLinksBroken(line, lineNum+1, sourceFile)
		brokenLinks = append(brokenLinks, broken...)

		// Check reference-style links
		brokenRef := checkReferenceLinksBroken(line, lineNum+1, sourceFile)
		brokenLinks = append(brokenLinks, brokenRef...)

		// Check image links
		brokenImg := checkImageLinksBroken(line, lineNum+1, sourceFile)
		brokenLinks = append(brokenLinks, brokenImg...)
	}

	return brokenLinks, nil
}

// isInsideInlineCode checks if a position in a line is inside inline code (backticks).
func isInsideInlineCode(line string, pos int) bool {
	backtickCount := 0
	for i := 0; i < pos && i < len(line); i++ {
		if line[i] == '`' {
			backtickCount++
		}
	}
	// If odd number of backticks before position, we're inside inline code
	return backtickCount%2 == 1
}

// checkInlineLinksBroken checks for broken inline links in a line.
func checkInlineLinksBroken(line string, lineNum int, sourceFile string) []BrokenLink {
	var broken []BrokenLink

	for i := range len(line) {
		if i+1 < len(line) && line[i] == ']' && line[i+1] == '(' {
			// Skip if this link is inside inline code
			if isInsideInlineCode(line, i) {
				continue
			}
			start := -1
			for j := i - 1; j >= 0; j-- {
				if line[j] == '[' {
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

			end := strings.Index(line[i+2:], ")")
			if end == -1 {
				continue
			}
			end += i + 2

			linkTarget := line[i+2 : end]

			// Skip external URLs
			if strings.HasPrefix(linkTarget, "http://") || strings.HasPrefix(linkTarget, "https://") {
				continue
			}

			// Remove fragment for file existence check
			targetPath := strings.Split(linkTarget, "#")[0]
			if targetPath == "" {
				continue // Fragment-only link (same page)
			}

			// Resolve and check if file exists
			resolved, err := resolveRelativePath(sourceFile, targetPath)
			if err != nil {
				continue
			}

			if !fileExists(resolved) {
				broken = append(broken, BrokenLink{
					SourceFile: sourceFile,
					LineNumber: lineNum,
					Target:     linkTarget,
					LinkType:   LinkTypeInline,
				})
			}
		}
	}

	return broken
}

// checkReferenceLinksBroken checks for broken reference-style links in a line.
func checkReferenceLinksBroken(line string, lineNum int, sourceFile string) []BrokenLink {
	var broken []BrokenLink

	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return broken
	}

	// Skip if the entire line is inside inline code
	if isInsideInlineCode(line, 0) {
		return broken
	}

	endBracket := strings.Index(trimmed, "]:")
	if endBracket == -1 {
		return broken
	}

	rest := strings.TrimSpace(trimmed[endBracket+2:])
	if rest == "" {
		return broken
	}

	linkTarget := rest
	if idx := strings.Index(rest, " \""); idx != -1 {
		linkTarget = rest[:idx]
	} else if idx := strings.Index(rest, " '"); idx != -1 {
		linkTarget = rest[:idx]
	}
	linkTarget = strings.TrimSpace(linkTarget)

	// Skip external URLs
	if strings.HasPrefix(linkTarget, "http://") || strings.HasPrefix(linkTarget, "https://") {
		return broken
	}

	// Remove fragment for file existence check
	targetPath := strings.Split(linkTarget, "#")[0]
	if targetPath == "" {
		return broken
	}

	// Resolve and check if file exists
	resolved, err := resolveRelativePath(sourceFile, targetPath)
	if err != nil {
		return broken
	}

	if !fileExists(resolved) {
		broken = append(broken, BrokenLink{
			SourceFile: sourceFile,
			LineNumber: lineNum,
			Target:     linkTarget,
			LinkType:   LinkTypeReference,
		})
	}

	return broken
}

// checkImageLinksBroken checks for broken image links in a line.
func checkImageLinksBroken(line string, lineNum int, sourceFile string) []BrokenLink {
	var broken []BrokenLink

	for i := range len(line) {
		if i+2 < len(line) && line[i] == '!' && line[i+1] == '[' {
			// Skip if this image link is inside inline code
			if isInsideInlineCode(line, i) {
				continue
			}
			closeBracket := strings.Index(line[i+2:], "]")
			if closeBracket == -1 {
				continue
			}
			closeBracket += i + 2

			if closeBracket+1 >= len(line) || line[closeBracket+1] != '(' {
				continue
			}

			end := strings.Index(line[closeBracket+2:], ")")
			if end == -1 {
				continue
			}
			end += closeBracket + 2

			linkTarget := line[closeBracket+2 : end]

			// Skip external URLs
			if strings.HasPrefix(linkTarget, "http://") || strings.HasPrefix(linkTarget, "https://") {
				continue
			}

			// Resolve and check if file exists
			resolved, err := resolveRelativePath(sourceFile, linkTarget)
			if err != nil {
				continue
			}

			if !fileExists(resolved) {
				broken = append(broken, BrokenLink{
					SourceFile: sourceFile,
					LineNumber: lineNum,
					Target:     linkTarget,
					LinkType:   LinkTypeImage,
				})
			}
		}
	}

	return broken
}

// fileExists checks if a file exists (case-insensitive on applicable filesystems).
func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}

	// On case-insensitive filesystems, try case-insensitive lookup
	// by checking the directory listing
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), base) {
			return true
		}
	}

	return false
}

// pathsEqualCaseInsensitive compares two paths case-insensitively.
// This is important for filesystems like macOS (HFS+/APFS) and Windows (NTFS)
// that are case-insensitive but case-preserving.
func pathsEqualCaseInsensitive(path1, path2 string) bool {
	// Normalize paths to forward slashes for consistent comparison
	path1 = filepath.ToSlash(filepath.Clean(path1))
	path2 = filepath.ToSlash(filepath.Clean(path2))

	// Case-insensitive comparison
	return strings.EqualFold(path1, path2)
}

// resolveRelativePath resolves a relative link path from a source file to an absolute path.
func resolveRelativePath(sourceFile, linkTarget string) (string, error) {
	// Remove any URL fragments (#section)
	targetPath := strings.Split(linkTarget, "#")[0]

	var resolvedPath string

	// Handle absolute paths (e.g., /local/docs/api-guide)
	// These are Hugo site-absolute paths that need to be resolved relative to content root
	if strings.HasPrefix(targetPath, "/") {
		// Find the content root by walking up from source file
		contentRoot := findContentRoot(sourceFile)
		if contentRoot != "" {
			// Strip leading slash and join with content root
			targetPath = strings.TrimPrefix(targetPath, "/")
			resolvedPath = filepath.Join(contentRoot, targetPath)
		} else {
			// Fallback: treat as filesystem absolute path
			resolvedPath = targetPath
		}
	} else {
		// Relative path - resolve relative to source file directory
		sourceDir := filepath.Dir(sourceFile)
		resolvedPath = filepath.Join(sourceDir, targetPath)
	}

	cleanPath := filepath.Clean(resolvedPath)

	// Try with .md extension if file doesn't exist as-is
	if !fileExists(cleanPath) {
		// Try adding .md extension (Hugo strips .md from URLs)
		withMd := cleanPath + ".md"
		if fileExists(withMd) {
			return withMd, nil
		}
		// Try adding .markdown extension
		withMarkdown := cleanPath + ".markdown"
		if fileExists(withMarkdown) {
			return withMarkdown, nil
		}
	}

	// Get absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return absPath, nil
}

// findContentRoot finds the content directory by walking up from the source file.
// It looks for a directory named "content" in the path hierarchy.
func findContentRoot(sourceFile string) string {
	dir := filepath.Dir(sourceFile)
	for {
		if filepath.Base(dir) == "content" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding content directory
			return ""
		}
		dir = parent
	}
}

// findInlineLinks finds inline-style markdown links: [text](path).
func findInlineLinks(line string, lineNum int, sourceFile, targetPath string) []LinkReference {
	// Pattern: [text](path) or [text](path#fragment)
	// We'll use a simple string search for now, can be improved with regex
	var links []LinkReference

	// Look for ]( pattern
	for i := range len(line) {
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

			// Check if this link points to our target (case-insensitive for filesystem compatibility)
			if pathsEqualCaseInsensitive(resolved, targetPath) {
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

// findReferenceLinks finds reference-style markdown links: [id]: path.
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

	// Check if this link points to our target (case-insensitive for filesystem compatibility)
	if pathsEqualCaseInsensitive(resolved, targetPath) {
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

// findImageLinks finds image markdown links: ![alt](path).
func findImageLinks(line string, lineNum int, sourceFile, targetPath string) []LinkReference {
	var links []LinkReference

	// Look for ![]( pattern
	for i := range len(line) {
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

			// Check if this link points to our target (case-insensitive for filesystem compatibility)
			if pathsEqualCaseInsensitive(resolved, targetPath) {
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
	var backupPaths []string

	// Process each file
	for sourceFile, linkRefs := range fileLinks {
		// Read file content
		content, err := os.ReadFile(sourceFile)
		if err != nil {
			// Rollback any previous changes
			f.rollbackLinkUpdates(backupPaths)
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
			err := os.WriteFile(backupPath, content, 0o644)
			if err != nil {
				// Rollback previous changes
				f.rollbackLinkUpdates(backupPaths)
				return nil, fmt.Errorf("failed to create backup for %s: %w", sourceFile, err)
			}
			backupPaths = append(backupPaths, backupPath)

			// Write updated content
			err = os.WriteFile(sourceFile, []byte(newContent), 0o644)
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
		newTarget = newTarget + link.Fragment
	}

	return newTarget
}

// rollbackLinkUpdates restores files from their backups.
// This provides transactional rollback on any failure during link updates.
func (f *Fixer) rollbackLinkUpdates(backupPaths []string) {
	for _, backupPath := range backupPaths {
		// Extract original file path by removing .backup suffix
		originalFile := strings.TrimSuffix(backupPath, ".backup")

		if content, err := os.ReadFile(backupPath); err == nil {
			_ = os.WriteFile(originalFile, content, 0o644) // Best effort restore
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

	if len(fr.BrokenLinks) > 0 {
		b.WriteString(fmt.Sprintf("\nBroken links detected: %d\n", len(fr.BrokenLinks)))
		b.WriteString("These links reference non-existent files:\n")
		for _, broken := range fr.BrokenLinks {
			linkTypeStr := "link"
			switch broken.LinkType {
			case LinkTypeImage:
				linkTypeStr = "image"
			case LinkTypeReference:
				linkTypeStr = "reference"
			}
			b.WriteString(fmt.Sprintf("  • %s:%d: %s → %s (broken %s)\n",
				broken.SourceFile, broken.LineNumber, linkTypeStr, broken.Target, linkTypeStr))
		}
	}

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

// PreviewChanges returns a detailed preview of what will be changed.
// This is shown to users before applying fixes for confirmation.
func (fr *FixResult) PreviewChanges() string {
	var b strings.Builder

	b.WriteString("The following changes will be made:\n\n")

	// File renames section
	if len(fr.FilesRenamed) > 0 {
		b.WriteString("FILE RENAMES:\n")
		for _, rename := range fr.FilesRenamed {
			oldName := filepath.Base(rename.OldPath)
			newName := filepath.Base(rename.NewPath)
			b.WriteString(fmt.Sprintf("  %s → %s\n", oldName, newName))
		}
		b.WriteString("\n")
	}

	// Links to update section
	if len(fr.LinksUpdated) > 0 {
		// Group by source file
		fileUpdates := make(map[string][]LinkUpdate)
		for _, update := range fr.LinksUpdated {
			filename := filepath.Base(update.SourceFile)
			fileUpdates[filename] = append(fileUpdates[filename], update)
		}

		b.WriteString("LINKS TO UPDATE:\n")
		for filename, updates := range fileUpdates {
			b.WriteString(fmt.Sprintf("  • %s (%d link%s)\n", filename, len(updates), pluralize(len(updates))))
		}
		b.WriteString("\n")
	}

	// Statistics
	b.WriteString("SUMMARY:\n")
	b.WriteString(fmt.Sprintf("  • %d file%s will be renamed\n", len(fr.FilesRenamed), pluralize(len(fr.FilesRenamed))))
	b.WriteString(fmt.Sprintf("  • %d link%s will be updated\n", len(fr.LinksUpdated), pluralize(len(fr.LinksUpdated))))
	if len(fr.BrokenLinks) > 0 {
		b.WriteString(fmt.Sprintf("  • %d broken link%s detected\n", len(fr.BrokenLinks), pluralize(len(fr.BrokenLinks))))
	}

	return b.String()
}

// DetailedPreview returns a line-by-line preview of what will change.
// This is shown in dry-run mode to see exact before/after states.
func (fr *FixResult) DetailedPreview() string {
	var b strings.Builder

	b.WriteString("DETAILED CHANGES PREVIEW\n")
	b.WriteString(strings.Repeat("=", 60) + "\n\n")

	// File renames with full paths
	if len(fr.FilesRenamed) > 0 {
		b.WriteString("[File Renames]\n")
		for i, rename := range fr.FilesRenamed {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, rename.OldPath))
			b.WriteString(fmt.Sprintf("   → %s\n\n", rename.NewPath))
		}
	}

	// Link updates with before/after
	if len(fr.LinksUpdated) > 0 {
		b.WriteString("[Link Updates]\n")
		for i, update := range fr.LinksUpdated {
			b.WriteString(fmt.Sprintf("%d. %s:%d\n", i+1, update.SourceFile, update.LineNumber))
			b.WriteString(fmt.Sprintf("   Before: %s\n", update.OldTarget))
			b.WriteString(fmt.Sprintf("   After:  %s\n\n", update.NewTarget))
		}
	}

	// Broken links section
	if len(fr.BrokenLinks) > 0 {
		b.WriteString("[Broken Links Detected]\n")
		for i, broken := range fr.BrokenLinks {
			linkTypeStr := "link"
			switch broken.LinkType {
			case LinkTypeImage:
				linkTypeStr = "image"
			case LinkTypeReference:
				linkTypeStr = "reference"
			}
			b.WriteString(fmt.Sprintf("%d. %s:%d (%s)\n", i+1, broken.SourceFile, broken.LineNumber, linkTypeStr))
			b.WriteString(fmt.Sprintf("   Target: %s (file not found)\n\n", broken.Target))
		}
	}

	b.WriteString(strings.Repeat("=", 60) + "\n")
	b.WriteString(fmt.Sprintf("Total: %d file%s, %d link%s\n",
		len(fr.FilesRenamed), pluralize(len(fr.FilesRenamed)),
		len(fr.LinksUpdated), pluralize(len(fr.LinksUpdated))))

	return b.String()
}

// HasChanges returns true if there are any fixes to apply.
func (fr *FixResult) HasChanges() bool {
	return len(fr.FilesRenamed) > 0 || len(fr.LinksUpdated) > 0
}

// CountAffectedFiles returns the number of unique files that will be modified.
func (fr *FixResult) CountAffectedFiles() int {
	affected := make(map[string]bool)

	// Files being renamed
	for _, rename := range fr.FilesRenamed {
		affected[rename.OldPath] = true
	}

	// Files with link updates
	for _, update := range fr.LinksUpdated {
		affected[update.SourceFile] = true
	}

	return len(affected)
}
