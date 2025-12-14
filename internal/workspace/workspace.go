package workspace

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// Manager handles workspace operations (both temporary and persistent)
type Manager struct {
	baseDir    string
	tempDir    string
	persistent bool // If true, use baseDir directly without timestamps
}

// NewManager creates a new workspace manager with ephemeral timestamped directories
func NewManager(baseDir string) *Manager {
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	return &Manager{
		baseDir:    baseDir,
		persistent: false,
	}
}

// NewPersistentManager creates a workspace manager that uses a persistent directory.
// The workspace directory is fixed (baseDir/subdirName) and not cleaned up on Cleanup().
func NewPersistentManager(baseDir, subdirName string) *Manager {
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	if subdirName == "" {
		subdirName = "working"
	}
	return &Manager{
		baseDir:    baseDir,
		tempDir:    filepath.Join(baseDir, subdirName),
		persistent: true,
	}
}

// Create creates a workspace directory
// For ephemeral mode: creates a timestamped directory
// For persistent mode: ensures the fixed directory exists
func (m *Manager) Create() error {
	if m.persistent {
		// Persistent mode: use fixed directory
		if err := os.MkdirAll(m.tempDir, 0o750); err != nil {
			return fmt.Errorf("failed to create persistent workspace directory: %w", err)
		}
		slog.Info("Using persistent workspace", logfields.Path(m.tempDir))
		return nil
	}

	// Ephemeral mode: create timestamped directory
	timestamp := time.Now().Format("20060102-150405")
	tempDir := filepath.Join(m.baseDir, fmt.Sprintf("docbuilder-%s", timestamp))

	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	m.tempDir = tempDir
	slog.Info("Created workspace", logfields.Path(tempDir))
	return nil
}

// GetPath returns the path to the workspace directory
func (m *Manager) GetPath() string {
	return m.tempDir
}

// Cleanup removes the workspace directory
// For persistent mode: does nothing (keeps directory for incremental builds)
// For ephemeral mode: removes the timestamped directory
func (m *Manager) Cleanup() error {
	if m.tempDir == "" {
		return nil
	}

	if m.persistent {
		// Persistent mode: don't delete the directory
		slog.Debug("Skipping cleanup for persistent workspace", logfields.Path(m.tempDir))
		return nil
	}

	// Ephemeral mode: remove directory
	if err := os.RemoveAll(m.tempDir); err != nil {
		return fmt.Errorf("failed to cleanup workspace: %w", err)
	}

	slog.Info("Cleaned up workspace", logfields.Path(m.tempDir))
	m.tempDir = ""
	return nil
}

// CreateSubdir creates a subdirectory within the workspace
func (m *Manager) CreateSubdir(name string) (string, error) {
	if m.tempDir == "" {
		return "", fmt.Errorf("workspace not created")
	}

	subdir := filepath.Join(m.tempDir, name)
	if err := os.MkdirAll(subdir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create subdirectory: %w", err)
	}

	return subdir, nil
}
