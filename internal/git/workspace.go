package git

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

func (c *Client) EnsureWorkspace() error {
	if err := os.MkdirAll(c.workspaceDir, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	return nil
}

func (c *Client) CleanWorkspace() error {
	entries, err := os.ReadDir(c.workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read workspace directory: %w", err)
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(c.workspaceDir, e.Name())); err != nil {
			return fmt.Errorf("remove %s: %w", e.Name(), err)
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
	return false, fmt.Errorf("failed to check .docignore file: %w", err)
}
