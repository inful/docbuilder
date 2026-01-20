package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFix_FileProcessingLoop tests the file processing loop with different scenarios.
// This tests the complexity 13 nested conditionals in the fix() method.

// TestFix_SuccessfulRenameWithLinkUpdates tests successful file rename with link updates.
func TestFix_SuccessfulRenameWithLinkUpdates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with uppercase letters (violates naming convention)
	badFile := filepath.Join(tmpDir, "BadFilename.md")
	if err := os.WriteFile(badFile, []byte("# Test"), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create a file that links to the bad filename
	linkingFile := filepath.Join(tmpDir, "linking.md")
	linkContent := []byte("[Link](BadFilename.md)")
	if err := os.WriteFile(linkingFile, linkContent, 0o600); err != nil {
		t.Fatalf("failed to create linking file: %v", err)
	}

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, true) // not dry-run, auto-fix enabled

	result, err := fixer.fix(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was renamed
	if len(result.FilesRenamed) != 1 {
		t.Errorf("expected 1 file renamed, got %d", len(result.FilesRenamed))
	}

	if len(result.FilesRenamed) > 0 {
		if !result.FilesRenamed[0].Success {
			t.Errorf("expected rename to succeed, got: %v", result.FilesRenamed[0].Error)
		}
	}

	// Verify link was updated
	if len(result.LinksUpdated) == 0 {
		t.Error("expected link to be updated")
	}
}

// TestFix_SuccessfulRenameNoLinks tests successful file rename without any links.
func TestFix_SuccessfulRenameNoLinks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with uppercase letters (violates naming convention)
	badFile := filepath.Join(tmpDir, "BadFilename.md")
	if err := os.WriteFile(badFile, []byte("# Test"), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, true)

	result, err := fixer.fix(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was renamed
	if len(result.FilesRenamed) != 1 {
		t.Errorf("expected 1 file renamed, got %d", len(result.FilesRenamed))
	}

	// Verify no link updates (no links exist)
	if len(result.LinksUpdated) != 0 {
		t.Errorf("expected 0 links updated, got %d", len(result.LinksUpdated))
	}

	// Verify errors fixed count
	if result.ErrorsFixed == 0 {
		t.Error("expected ErrorsFixed > 0")
	}
}

// TestFix_RenameFailure tests handling of rename operation failure.
func TestFix_RenameFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with uppercase letters
	badFile := filepath.Join(tmpDir, "BadFilename.md")
	if err := os.WriteFile(badFile, []byte("# Test"), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create the target file (will cause rename to fail)
	expectedTarget := filepath.Join(tmpDir, "badfilename.md")
	if err := os.WriteFile(expectedTarget, []byte("# Existing"), 0o600); err != nil {
		t.Fatalf("failed to create conflicting file: %v", err)
	}

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false) // force=false to fail on conflict

	result, err := fixer.fix(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify rename operation was attempted but failed
	if len(result.FilesRenamed) != 1 {
		t.Errorf("expected 1 rename operation, got %d", len(result.FilesRenamed))
	}

	if len(result.FilesRenamed) > 0 {
		if result.FilesRenamed[0].Success {
			t.Error("expected rename to fail due to conflict")
		}
	}

	// Verify error was recorded
	if len(result.Errors) == 0 {
		t.Error("expected error to be recorded")
	}

	// Even if rename fails, other fixable issues (like fingerprints) may still be applied.
	if result.ErrorsFixed == 0 {
		t.Errorf("expected ErrorsFixed > 0, got %d", result.ErrorsFixed)
	}
}

// TestFix_DryRunMode tests that dry-run mode doesn't update links.
func TestFix_DryRunMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	badFile := filepath.Join(tmpDir, "BadFilename.md")
	if err := os.WriteFile(badFile, []byte("# Test"), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	linkingFile := filepath.Join(tmpDir, "linking.md")
	linkContent := []byte("[Link](BadFilename.md)")
	if err := os.WriteFile(linkingFile, linkContent, 0o600); err != nil {
		t.Fatalf("failed to create linking file: %v", err)
	}

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, true, false) // dry-run mode, no auto-fix

	result, err := fixer.fix(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify rename was simulated
	if len(result.FilesRenamed) != 1 {
		t.Errorf("expected 1 file rename simulation, got %d", len(result.FilesRenamed))
	}

	// Verify no link updates in dry-run mode
	if len(result.LinksUpdated) != 0 {
		t.Errorf("expected 0 links updated in dry-run, got %d", len(result.LinksUpdated))
	}

	// Original file should still exist
	if _, err := os.Stat(badFile); os.IsNotExist(err) {
		t.Error("expected original file to still exist in dry-run")
	}
}

// TestFix_FindLinksError tests handling of findLinksToFile error.
func TestFix_FindLinksError(t *testing.T) {
	t.Skip("Skipping test that relies on filesystem permissions which are inconsistent across environments")
	tmpDir := t.TempDir()

	// Create a file to rename
	badFile := filepath.Join(tmpDir, "BadFilename.md")
	if err := os.WriteFile(badFile, []byte("# Test"), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create a directory with restricted permissions to trigger search errors
	restrictedDir := filepath.Join(tmpDir, "restricted")
	if err := os.Mkdir(restrictedDir, 0o000); err != nil {
		t.Fatalf("failed to create restricted dir: %v", err)
	}
	defer func() {
		// Restore permissions for cleanup
		// #nosec G302 -- intentional permission change for test cleanup
		_ = os.Chmod(restrictedDir, 0o755)
	}()

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, true)

	result, err := fixer.fix(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be renamed successfully
	if len(result.FilesRenamed) == 0 || !result.FilesRenamed[0].Success {
		t.Error("expected file to be renamed successfully")
	}

	// Link search error should be recorded (non-fatal)
	// Note: This may or may not trigger depending on the OS and permissions
	// The test verifies the function completes even if link search fails
}

// TestFix_ApplyLinkUpdatesError tests handling of applyLinkUpdates error.
func TestFix_ApplyLinkUpdatesError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file to rename
	badFile := filepath.Join(tmpDir, "BadFilename.md")
	if err := os.WriteFile(badFile, []byte("# Test"), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create a read-only linking file to cause update error
	linkingFile := filepath.Join(tmpDir, "readonly.md")
	linkContent := []byte("[Link](BadFilename.md)")
	// #nosec G306 -- intentional read-only permission for test setup
	if err := os.WriteFile(linkingFile, linkContent, 0o444); err != nil {
		t.Fatalf("failed to create linking file: %v", err)
	}
	defer func() {
		// Restore write permissions for cleanup
		// #nosec G302 -- intentional permission change for test cleanup
		_ = os.Chmod(linkingFile, 0o600)
	}()

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, true)

	result, err := fixer.fix(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be renamed successfully
	if len(result.FilesRenamed) == 0 || !result.FilesRenamed[0].Success {
		t.Error("expected file to be renamed successfully")
	}

	// Link update error should be recorded
	if len(result.Errors) == 0 {
		t.Error("expected link update error to be recorded")
	}
}

// TestFix_NoFilenameIssues tests when there are no filename issues to fix.
func TestFix_NoFilenameIssues(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with valid name
	goodFile := filepath.Join(tmpDir, "valid-filename.md")
	if err := os.WriteFile(goodFile, []byte("# Test"), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, true)

	result, err := fixer.fix(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No files should be renamed
	if len(result.FilesRenamed) != 0 {
		t.Errorf("expected 0 files renamed, got %d", len(result.FilesRenamed))
	}

	// Fingerprints are still fixed even when there are no filename issues.
	if result.ErrorsFixed == 0 {
		t.Errorf("expected ErrorsFixed > 0, got %d", result.ErrorsFixed)
	}
}
