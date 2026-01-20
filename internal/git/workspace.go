package git

import (
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

func (c *Client) EnsureWorkspace() error {
	if err := os.MkdirAll(c.workspaceDir, 0o750); err != nil {
		return errors.NewError(errors.CategoryFileSystem, "failed to create workspace directory").
			WithCause(err).
			WithContext("path", c.workspaceDir).
			Build()
	}
	return nil
}

func (c *Client) CleanWorkspace() error {
	entries, err := os.ReadDir(c.workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.NewError(errors.CategoryFileSystem, "failed to read workspace directory").
			WithCause(err).
			WithContext("path", c.workspaceDir).
			Build()
	}
	for _, e := range entries {
		path := filepath.Join(c.workspaceDir, e.Name())
		if err := os.RemoveAll(path); err != nil {
			return errors.NewError(errors.CategoryFileSystem, "failed to remove workspace entry").
				WithCause(err).
				WithContext("path", path).
				Build()
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
	return false, errors.NewError(errors.CategoryFileSystem, "failed to check .docignore file").
		WithCause(err).
		WithContext("path", path).
		Build()
}
