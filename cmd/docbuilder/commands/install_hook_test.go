package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/lint"
)

// The hook lints the staged files where they are.
//
// A copy of them in a temporary directory has no neighbors, and every
// link to a file that was not staged is a broken link: the hook
// rejected the commit of every documentation that links to a file of
// its own repository.
func TestTheHookLintsTheStagedFilesInPlace(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	docs := filepath.Join(repo, "docs")
	if err := os.MkdirAll(docs, 0o750); err != nil {
		t.Fatalf("make the docs dir: %v", err)
	}

	// A page that links to a neighbor.
	page := filepath.Join(docs, "guide.md")
	if err := os.WriteFile(page, []byte("# Guide\n\nSee the [setup](setup.md).\n"), 0o600); err != nil {
		t.Fatalf("write the page: %v", err)
	}
	// The neighbor is not staged.
	if err := os.WriteFile(filepath.Join(docs, "setup.md"), []byte("# Setup\n"), 0o600); err != nil {
		t.Fatalf("write the neighbor: %v", err)
	}

	// The linter is asked for the file where it is.
	linter := lint.NewLinter(&lint.Config{})
	result, err := linter.LintPath(page)
	if err != nil {
		t.Fatalf("the linter failed: %v", err)
	}

	for _, issue := range result.Issues {
		if issue.Rule == "broken-links" {
			t.Fatalf("the link to the neighbor was reported as broken: %s", issue.Message)
		}
	}

	// A copy of the page alone has no neighbors: the link is broken.
	// This is what the hook did.
	alone := filepath.Join(t.TempDir(), "guide.md")
	//nolint:gosec // the path is the test's own
	content, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("read the page: %v", err)
	}
	//nolint:gosec // the path is the test's own
	if writeErr := os.WriteFile(alone, content, 0o600); writeErr != nil {
		t.Fatalf("write the copy: %v", writeErr)
	}

	copied, err := lint.NewLinter(&lint.Config{}).LintPath(alone)
	if err != nil {
		t.Fatalf("the linter failed: %v", err)
	}

	broken := false
	for _, issue := range copied.Issues {
		if issue.Rule == "broken-links" {
			broken = true
		}
	}
	if !broken {
		t.Fatal("the copy of the page did not have a broken link: the test does not show the defect")
	}
}

// The hook does not copy the staged files into a temporary directory.
func TestTheHookDoesNotCopyTheStagedFiles(t *testing.T) {
	hook := installHookScript()

	if strings.Contains(hook, "TEMP_DIR") {
		t.Fatal("the hook copies the staged files into a temporary directory")
	}
	if !strings.Contains(hook, `lint "$file"`) {
		t.Fatal("the hook does not lint the staged files where they are")
	}
}
