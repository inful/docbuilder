package lint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

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

// shouldUseGitMv checks if a file is under Git version control and tracked.
func (f *Fixer) shouldUseGitMv(_ string) bool {
	if f.gitRepo == nil {
		return false
	}

	// For now, let's use a simpler check: if we have a gitRepo, we'll try gitMv and fallback if it fails.
	// In the future, we could check if the file is tracked in the head commit.
	return true
}

// gitMv performs a git mv operation using the Git library.
func (f *Fixer) gitMv(oldPath, newPath string) error {
	if f.gitRepo == nil {
		return errors.New("git repository not initialized")
	}

	w, err := f.gitRepo.Worktree()
	if err != nil {
		return err
	}

	// Calculate paths relative to the repository root
	// We assume f.gitRepo was opened at the root of the workspace being fixed.

	// Perform the move
	_, err = w.Move(oldPath, newPath)
	return err
}

// isGitRepository checks if the given directory is a Git repository.
func isGitRepository(dir string) bool {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// isGitClean checks if the Git repository has uncommitted changes.
func (f *Fixer) isGitClean(_ string) (bool, error) {
	if f.gitRepo == nil {
		return true, nil
	}

	w, err := f.gitRepo.Worktree()
	if err != nil {
		return false, err
	}

	status, err := w.Status()
	if err != nil {
		return false, err
	}

	return status.IsClean(), nil
}

// rollback reverts all changes made during the fixer session using Git.
func (f *Fixer) rollback() error {
	if f.gitRepo == nil || f.dryRun {
		return nil
	}

	w, err := f.gitRepo.Worktree()
	if err != nil {
		return err
	}

	// Reset to initial SHA
	err = w.Reset(&git.ResetOptions{
		Commit: f.initialSHA,
		Mode:   git.HardReset,
	})
	if err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}

	return nil
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
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o750); err != nil {
		return err
	}

	// Copy file
	return f.copyFile(filePath, backupPath)
}

// copyFile copies a file from src to dst.
func (f *Fixer) copyFile(src, dst string) error {
	// #nosec G304 -- src/dst are validated file paths from fixer operations
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	// #nosec G304 -- src/dst are validated file paths from fixer operations
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
