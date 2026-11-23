package lint

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The git commands of the fixer run in the repository of the file that
// is fixed and not in the working directory of the process: an MCP
// server started in another repository than the docs directory renamed
// the wrong file.
func TestTheGitCommandsRunInTheRepositoryOfTheFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	// The repository of the documentation.
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o750); err != nil {
		t.Fatalf("make the docs dir: %v", err)
	}
	doc := filepath.Join(repo, "docs", "Guide.md")
	if err := os.WriteFile(doc, []byte("# Guide\n"), 0o600); err != nil {
		t.Fatalf("write the doc: %v", err)
	}
	for _, args := range [][]string{{"init"}, {"add", "docs/Guide.md"}} {
		//nolint:gosec // the command is built from constants
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Another repository, in the working directory of the process.
	other := t.TempDir()
	//nolint:gosec // the command is built from constants
	if out, err := exec.CommandContext(t.Context(), "git", "-C", other, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	t.Chdir(other)

	fixer := NewFixer(nil, false, false)
	if !fixer.shouldUseGitMv(doc) {
		t.Fatal("the file of the repository of the documentation was not found")
	}
}
