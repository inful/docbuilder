package lint

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	linkTypeInline    = "link"
	linkTypeImage     = "image"
	linkTypeReference = "reference"
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
		f.processFileWithIssues(filePath, issues, rootPath, fixResult)
	}

	return fixResult, nil
}

// processFileWithIssues handles fixing issues for a single file.
func (f *Fixer) processFileWithIssues(filePath string, issues []Issue, rootPath string, fixResult *FixResult) {
	// Check if this is a filename issue that can be fixed
	if !f.canFixFilename(issues) {
		return
	}

	op := f.renameFile(filePath)
	fixResult.FilesRenamed = append(fixResult.FilesRenamed, op)

	if op.Success {
		fixResult.ErrorsFixed += len(issues)
		f.handleSuccessfulRename(filePath, op.NewPath, rootPath, fixResult)
	} else if op.Error != nil {
		fixResult.Errors = append(fixResult.Errors, op.Error)
	}
}

// handleSuccessfulRename processes link updates after a successful file rename.
func (f *Fixer) handleSuccessfulRename(oldPath, newPath, rootPath string, fixResult *FixResult) {
	// Skip link updates in dry-run mode
	if f.dryRun {
		return
	}

	updates, err := f.findAndUpdateLinks(oldPath, newPath, rootPath)
	if err != nil {
		fixResult.Errors = append(fixResult.Errors, err)
		return
	}

	fixResult.LinksUpdated = append(fixResult.LinksUpdated, updates...)
}

// findAndUpdateLinks finds all links to a file and updates them to the new path.
func (f *Fixer) findAndUpdateLinks(oldPath, newPath, rootPath string) ([]LinkUpdate, error) {
	links, err := f.findLinksToFile(oldPath, rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find links to %s: %w", oldPath, err)
	}

	if len(links) == 0 {
		return nil, nil
	}

	updates, err := f.applyLinkUpdates(links, oldPath, newPath)
	if err != nil {
		return nil, fmt.Errorf("failed to update links: %w", err)
	}

	return updates, nil
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

	if err := os.MkdirAll(backupDir, 0o750); err != nil {
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
