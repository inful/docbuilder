package lint

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/inful/mdfp"
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
		Fingerprints: make([]FingerprintUpdate, 0),
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

	// Group issues by file and track uid/fingerprint fix targets.
	// IMPORTANT: fingerprint regeneration must always run as the final fix step,
	// because other fixes (like uid insertion and link rewrites) can change content
	// and would invalidate previously-computed fingerprints.
	fileIssues := make(map[string][]Issue)
	fingerprintTargets := make(map[string]struct{})
	uidTargets := make(map[string]struct{})
	uidAliasTargets := make(map[string]struct{})
	uidIssueCounts := make(map[string]int)
	uidAliasIssueCounts := make(map[string]int)
	fingerprintIssueCounts := make(map[string]int)
	for _, issue := range result.Issues {
		if issue.Severity != SeverityError {
			continue
		}
		fileIssues[issue.FilePath] = append(fileIssues[issue.FilePath], issue)
		if issue.Rule == frontmatterUIDRuleName && issue.Message == missingUIDMessage {
			uidTargets[issue.FilePath] = struct{}{}
			uidIssueCounts[issue.FilePath]++
		}
		if issue.Rule == frontmatterUIDRuleName && issue.Message == missingUIDaliasMessage {
			uidAliasTargets[issue.FilePath] = struct{}{}
			uidAliasIssueCounts[issue.FilePath]++
		}
		if issue.Rule == frontmatterFingerprintRuleName {
			fingerprintTargets[issue.FilePath] = struct{}{}
			fingerprintIssueCounts[issue.FilePath]++
		}
	}
	// Phase 1: add missing frontmatter uids (and corresponding aliases).
	// We never rewrite an existing uid (even if invalid), because uid must be stable.
	f.applyUIDFixes(uidTargets, uidIssueCounts, fixResult, fingerprintTargets)

	// Phase 2: add missing uid-based aliases (for files that already have valid uids).
	f.applyUIDAliasesFixes(uidAliasTargets, uidAliasIssueCounts, fixResult, fingerprintTargets)

	// Phase 3: perform renames + link updates.
	for filePath, issues := range fileIssues {
		f.processFileWithIssues(filePath, issues, rootPath, fixResult, fingerprintTargets, fingerprintIssueCounts)
	}

	// Phase 4: regenerate fingerprints LAST, for all affected files.
	// (This must remain the final fixer phase.)
	f.applyFingerprintFixes(fingerprintTargets, fingerprintIssueCounts, fixResult)

	return fixResult, nil
}

// processFileWithIssues handles fixing issues for a single file.
func (f *Fixer) processFileWithIssues(filePath string, issues []Issue, rootPath string, fixResult *FixResult, fingerprintTargets map[string]struct{}, fingerprintIssueCounts map[string]int) {
	currentPath := filePath

	// 1) Filename fixes (renames)
	if !f.canFixFilename(issues) {
		return
	}

	op := f.renameFile(filePath)
	fixResult.FilesRenamed = append(fixResult.FilesRenamed, op)
	if !op.Success {
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
		}
		return
	}

	// Only count issues we actually addressed: filename issues are always fixable.
	fixResult.ErrorsFixed += countIssuesByRule(issues, "filename-conventions")

	// In dry-run mode the rename is not applied, so subsequent operations must
	// continue to reference the original on-disk path.
	if !f.dryRun {
		currentPath = op.NewPath
	}

	// If this file needs a fingerprint fix, make sure we track the final path
	// (renames are applied on-disk only when not in dry-run).
	if _, ok := fingerprintTargets[filePath]; ok {
		delete(fingerprintTargets, filePath)
		fingerprintTargets[currentPath] = struct{}{}
		if c, okc := fingerprintIssueCounts[filePath]; okc {
			delete(fingerprintIssueCounts, filePath)
			fingerprintIssueCounts[currentPath] = c
		}
	}

	updates, err := f.handleSuccessfulRename(filePath, op.NewPath, rootPath)
	if err != nil {
		fixResult.Errors = append(fixResult.Errors, err)
		return
	}

	fixResult.LinksUpdated = append(fixResult.LinksUpdated, updates...)
	// Link rewrites modify content, so those files must have fingerprints refreshed.
	for _, upd := range updates {
		fingerprintTargets[upd.SourceFile] = struct{}{}
	}

	// Fingerprint fixes are intentionally deferred until AFTER all renames/link updates.
}

// handleSuccessfulRename processes link updates after a successful file rename.
func (f *Fixer) handleSuccessfulRename(oldPath, newPath, rootPath string) ([]LinkUpdate, error) {
	// Skip link updates in dry-run mode
	if f.dryRun {
		return nil, nil
	}

	updates, err := f.findAndUpdateLinks(oldPath, newPath, rootPath)
	if err != nil {
		return nil, err
	}

	return updates, nil
}

func (f *Fixer) applyFingerprintFixes(targets map[string]struct{}, fingerprintIssueCounts map[string]int, fixResult *FixResult) {
	if len(targets) == 0 {
		return
	}

	// De-dup and keep deterministic ordering in results.
	paths := make([]string, 0, len(targets))
	for p := range targets {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		// Only fingerprint markdown files.
		ext := strings.ToLower(filepath.Ext(p))
		if ext != docExtensionMarkdown && ext != docExtensionMarkdownLong {
			continue
		}

		fpOp := f.updateFrontmatterFingerprint(p)
		fixResult.Fingerprints = append(fixResult.Fingerprints, fpOp)
		if fpOp.Success {
			// Only count errors that were reported by the initial lint run.
			// Fingerprints may also be refreshed for files we modified (e.g. link rewrites),
			// but those invalidations occur after the initial lint pass.
			fixResult.ErrorsFixed += fingerprintIssueCounts[p]
		} else if fpOp.Error != nil {
			fixResult.Errors = append(fixResult.Errors, fpOp.Error)
		}
	}
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

func (f *Fixer) updateFrontmatterFingerprint(filePath string) FingerprintUpdate {
	op := FingerprintUpdate{FilePath: filePath, Success: true}

	// #nosec G304 -- filePath is derived from the current lint/fix target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for fingerprint update: %w", err)
		return op
	}

	updated, err := mdfp.ProcessContent(string(data))
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("compute fingerprint update: %w", err)
		return op
	}

	// mdfp may rewrite frontmatter; ensure stable uid is preserved.
	updated = preserveUIDAcrossContentRewrite(string(data), updated)

	if updated == string(data) {
		return op
	}

	if f.dryRun {
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for fingerprint update: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, []byte(updated), info.Mode().Perm()); writeErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("write file for fingerprint update: %w", writeErr)
		return op
	}

	return op
}

func countIssuesByRule(issues []Issue, rule string) int {
	count := 0
	for _, issue := range issues {
		if issue.Rule == rule {
			count++
		}
	}
	return count
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
