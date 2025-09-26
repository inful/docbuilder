package workspace

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Manager handles temporary workspace operations
type Manager struct {
	baseDir string
	tempDir string
}

// NewManager creates a new workspace manager
func NewManager(baseDir string) *Manager {
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	return &Manager{
		baseDir: baseDir,
	}
}

// Create creates a temporary workspace directory
func (m *Manager) Create() error {
	timestamp := time.Now().Format("20060102-150405")
	tempDir := filepath.Join(m.baseDir, fmt.Sprintf("docbuilder-%s", timestamp))

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	m.tempDir = tempDir
	slog.Info("Created workspace", "path", tempDir)
	return nil
}

// GetPath returns the path to the workspace directory
func (m *Manager) GetPath() string {
	return m.tempDir
}

// Cleanup removes the temporary workspace directory
func (m *Manager) Cleanup() error {
	if m.tempDir == "" {
		return nil
	}

	if err := os.RemoveAll(m.tempDir); err != nil {
		return fmt.Errorf("failed to cleanup workspace: %w", err)
	}

	slog.Info("Cleaned up workspace", "path", m.tempDir)
	m.tempDir = ""
	return nil
}

// CreateSubdir creates a subdirectory within the workspace
func (m *Manager) CreateSubdir(name string) (string, error) {
	if m.tempDir == "" {
		return "", fmt.Errorf("workspace not created")
	}

	subdir := filepath.Join(m.tempDir, name)
	if err := os.MkdirAll(subdir, 0755); err != nil {
		return "", fmt.Errorf("failed to create subdirectory: %w", err)
	}

	return subdir, nil
}
