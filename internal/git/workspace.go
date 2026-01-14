package git

import (
	"log/slog"
	"os"
	"path/filepath"

	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

func (c *Client) EnsureWorkspace() error {
	if err := os.MkdirAll(c.workspaceDir, 0o750); err != nil {
		return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to create workspace directory").WithSeverity(foundationerrors.SeverityError).WithContext("workspace_dir", c.workspaceDir).Build()
	}
	return nil
}

func (c *Client) CleanWorkspace() error {
	entries, err := os.ReadDir(c.workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to read workspace directory").WithSeverity(foundationerrors.SeverityError).WithContext("workspace_dir", c.workspaceDir).Build()
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(c.workspaceDir, e.Name())); err != nil {
			return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to remove workspace entry").WithSeverity(foundationerrors.SeverityError).WithContext("entry_name", e.Name()).Build()
		}
	}
	slog.Info("Workspace cleaned", logfields.Path(c.workspaceDir))
	return nil
}

func (c *Client) CheckDocIgnore(repoPath string) (bool, error) {
	path := filepath.Join(repoPath, ".docignore")
	_, err := os.Stat(path)
	if err == nil {
		slog.Debug("Found .docignore file", logfields.Path(path))
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to check .docignore file").WithSeverity(foundationerrors.SeverityError).WithContext("file_path", path).Build()
}
