package lint

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
		op.Error = foundationerrors.NewError(foundationerrors.CategoryValidation,
			"could not determine suggested filename or file is already correct").
			WithContext("file", oldPath).
			WithContext("current_name", filename).
			Build()
		return op
	}

	// Construct new path
	dir := filepath.Dir(oldPath)
	newPath := filepath.Join(dir, suggestedName)
	op.NewPath = newPath

	// Check if target already exists
	if _, err := os.Stat(newPath); err == nil && !f.force {
		op.Error = foundationerrors.NewError(foundationerrors.CategoryValidation,
			"target file already exists").
			WithContext("target_path", newPath).
			WithContext("source_path", oldPath).
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
			op.Error = foundationerrors.WrapError(err, foundationerrors.CategoryGit,
				"git mv failed").
				WithContext("old_path", oldPath).
				WithContext("new_path", newPath).
				Build()
			return op
		}
	} else {
		// Use regular file system rename
		err := os.Rename(oldPath, newPath)
		if err != nil {
			op.Error = foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
				"rename failed").
				WithContext("old_path", oldPath).
				WithContext("new_path", newPath).
				Build()
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
	cmd := exec.CommandContext(context.Background(), "git", "ls-files", "--error-unmatch", filePath)
	err := cmd.Run()
	return err == nil
}

// gitMv performs a git mv operation.
func (f *Fixer) gitMv(oldPath, newPath string) error {
	cmd := exec.CommandContext(context.Background(), "git", "mv", oldPath, newPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return foundationerrors.WrapError(err, foundationerrors.CategoryGit,
			"git mv command failed").
			WithContext("old_path", oldPath).
			WithContext("new_path", newPath).
			WithContext("output", string(output)).
			Build()
	}
	return nil
}

// isGitRepository checks if the given directory is a Git repository.
func isGitRepository(dir string) bool {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
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
