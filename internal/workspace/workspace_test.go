package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManager_EphemeralMode(t *testing.T) {
	tempBase := t.TempDir()
	mgr := NewManager(tempBase)

	// Create workspace
	if err := mgr.Create(); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify workspace exists and has timestamp
	wsPath := mgr.GetPath()
	if wsPath == "" {
		t.Fatal("GetPath() returned empty string")
	}

	if !strings.Contains(filepath.Base(wsPath), "docbuilder-") {
		t.Errorf("Expected timestamped directory, got: %s", wsPath)
	}

	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Errorf("Workspace directory does not exist: %s", wsPath)
	}

	// Cleanup should remove directory
	if err := mgr.Cleanup(); err != nil {
		t.Fatalf("Cleanup() failed: %v", err)
	}

	if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
		t.Errorf("Workspace directory still exists after cleanup: %s", wsPath)
	}
}

func TestManager_PersistentMode(t *testing.T) {
	tempBase := t.TempDir()
	mgr := NewPersistentManager(tempBase, "working")

	// Create workspace
	if err := mgr.Create(); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify workspace exists with fixed name
	wsPath := mgr.GetPath()
	expectedPath := filepath.Join(tempBase, "working")

	if wsPath != expectedPath {
		t.Errorf("Expected path %s, got: %s", expectedPath, wsPath)
	}

	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Errorf("Workspace directory does not exist: %s", wsPath)
	}

	// Create a marker file to verify persistence
	markerFile := filepath.Join(wsPath, "marker.txt")
	if err := os.WriteFile(markerFile, []byte("persistent"), 0o600); err != nil {
		t.Fatalf("Failed to create marker file: %v", err)
	}

	// Cleanup should NOT remove directory in persistent mode
	if err := mgr.Cleanup(); err != nil {
		t.Fatalf("Cleanup() failed: %v", err)
	}

	// Verify directory and marker still exist
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Errorf("Persistent workspace was removed: %s", wsPath)
	}

	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Errorf("Marker file was removed from persistent workspace")
	}
}

func TestManager_PersistentModeMultipleCreates(t *testing.T) {
	tempBase := t.TempDir()
	mgr := NewPersistentManager(tempBase, "working")

	// First create
	if err := mgr.Create(); err != nil {
		t.Fatalf("First Create() failed: %v", err)
	}

	wsPath := mgr.GetPath()
	markerFile := filepath.Join(wsPath, "marker.txt")
	if err := os.WriteFile(markerFile, []byte("test"), 0o600); err != nil {
		t.Fatalf("Failed to create marker file: %v", err)
	}

	// Second create on same manager
	mgr2 := NewPersistentManager(tempBase, "working")
	if err := mgr2.Create(); err != nil {
		t.Fatalf("Second Create() failed: %v", err)
	}

	// Marker file should still exist
	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Errorf("Marker file was removed by second Create()")
	}

	// Path should be the same
	if mgr2.GetPath() != wsPath {
		t.Errorf("Second manager has different path: %s vs %s", mgr2.GetPath(), wsPath)
	}
}

func TestManager_DefaultSubdirName(t *testing.T) {
	tempBase := t.TempDir()
	mgr := NewPersistentManager(tempBase, "")

	if err := mgr.Create(); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Should default to "working"
	expectedPath := filepath.Join(tempBase, "working")
	if mgr.GetPath() != expectedPath {
		t.Errorf("Expected default subdir 'working', got: %s", mgr.GetPath())
	}
}
