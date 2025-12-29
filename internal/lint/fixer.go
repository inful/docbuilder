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

// Summary returns a human-readable summary of the fix operation.
func (fr *FixResult) Summary() string {
	var b strings.Builder
	
	b.WriteString(fmt.Sprintf("Files renamed: %d\n", len(fr.FilesRenamed)))
	b.WriteString(fmt.Sprintf("Errors fixed: %d\n", fr.ErrorsFixed))
	
	if len(fr.Errors) > 0 {
		b.WriteString(fmt.Sprintf("\nErrors encountered: %d\n", len(fr.Errors)))
		for _, err := range fr.Errors {
			b.WriteString(fmt.Sprintf("  • %v\n", err))
		}
	}
	
	return b.String()
}
