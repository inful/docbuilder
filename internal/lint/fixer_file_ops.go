package lint

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
		op.Error = errors.NewError(errors.CategoryValidation, "could not determine suggested filename or file is already correct").Build()
		return op
	}

	// Construct new path
	dir := filepath.Dir(oldPath)
	newPath := filepath.Join(dir, suggestedName)
	op.NewPath = newPath

	// Check if target already exists
	if _, err := os.Stat(newPath); err == nil && !f.force {
		op.Error = errors.NewError(errors.CategoryAlreadyExists, "target file already exists").
			WithContext("path", newPath).
			Build()
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
			op.Error = errors.WrapError(err, errors.CategoryGit, "git mv failed").Build()
			return op
		}
	} else {
		// Use regular file system rename
		err := os.Rename(oldPath, newPath)
		if err != nil {
			op.Error = errors.WrapError(err, errors.CategoryFileSystem, "rename failed").Build()
			return op
		}
	}

	op.Success = true
	return op
}

// shouldUseGitMv checks if a file is under Git version control and tracked.
func (f *Fixer) shouldUseGitMv(filePath string) bool {
	if f.gitRepo == nil {
		return false
	}

	// Use the git index to check if the file is tracked.
	// Only tracked files can be moved with 'git mv'.
	idx, err := f.gitRepo.Storer.Index()
	if err != nil {
		return false
	}

	// Git paths in the index are always relative to the repository root and use forward slashes.
	// We assume f.gitRepo was opened at the root of the workspace being fixed (.).
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}
	root, err := filepath.Abs(".")
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(root, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return false // Path is outside repository
	}

	relPath = filepath.ToSlash(relPath)
	_, err = idx.Entry(relPath)
	return err == nil
}

// gitMv performs a git mv operation using the Git library.
func (f *Fixer) gitMv(oldPath, newPath string) error {
	if f.gitRepo == nil {
		return errors.NewError(errors.CategoryGit, "git repository not initialized").Build()
	}

	w, err := f.gitRepo.Worktree()
	if err != nil {
		return errors.WrapError(err, errors.CategoryGit, "failed to get git worktree").Build()
	}

	// Calculate paths relative to the repository root (.)
	root, err := filepath.Abs(".")
	if err != nil {
		return errors.WrapError(err, errors.CategoryFileSystem, "failed to get absolute path of current directory").Build()
	}

	absOld, err := filepath.Abs(oldPath)
	if err != nil {
		return errors.WrapError(err, errors.CategoryFileSystem, "failed to get absolute path of old file").Build()
	}
	relOld, err := filepath.Rel(root, absOld)
	if err != nil {
		return errors.WrapError(err, errors.CategoryFileSystem, "failed to get relative path of old file").Build()
	}

	absNew, err := filepath.Abs(newPath)
	if err != nil {
		return errors.WrapError(err, errors.CategoryFileSystem, "failed to get absolute path of new file").Build()
	}
	relNew, err := filepath.Rel(root, absNew)
	if err != nil {
		return errors.WrapError(err, errors.CategoryFileSystem, "failed to get relative path of new file").Build()
	}

	// Perform the move using repository-relative paths
	_, err = w.Move(filepath.ToSlash(relOld), filepath.ToSlash(relNew))
	if err != nil {
		return errors.WrapError(err, errors.CategoryGit, "failed to move file in git").Build()
	}

	return nil
}

// isGitRepository checks if the given directory is a Git repository.
func isGitRepository(dir string) bool {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// isGitClean checks if the Git repository has uncommitted changes.
func (f *Fixer) isGitClean() (bool, error) {
	if f.gitRepo == nil {
		return true, nil
	}

	w, err := f.gitRepo.Worktree()
	if err != nil {
		return false, errors.WrapError(err, errors.CategoryGit, "failed to get git worktree").Build()
	}

	status, err := w.Status()
	if err != nil {
		return false, errors.WrapError(err, errors.CategoryGit, "failed to get git status").Build()
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
		return errors.WrapError(err, errors.CategoryGit, "failed to get git worktree").Build()
	}

	// Reset to initial SHA
	err = w.Reset(&git.ResetOptions{
		Commit: f.initialSHA,
		Mode:   git.HardReset,
	})
	if err != nil {
		return errors.WrapError(err, errors.CategoryGit, "git reset failed").Build()
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
