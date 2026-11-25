package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The hook has to land where git reads it: the command is run from the
// directory of the documentation, and the path of the git directory
// ended with the newline of the output of the command. The hook was
// written to a directory that does not exist, and the command said it
// was installed.
func TestTheHookIsInstalledWhereGitReadsIt(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatalf("make the repo: %v", err)
	}
	runGit(t, repo, "init", "-q")

	// The command is run from the directory of the documentation, not
	// from the root of the repository.
	docs := filepath.Join(repo, "docs")
	if err := os.MkdirAll(docs, 0o750); err != nil {
		t.Fatalf("make the docs dir: %v", err)
	}
	t.Chdir(docs)

	cmd := &InstallHookCmd{}
	if err := cmd.Run(nil, nil); err != nil {
		t.Fatalf("install the hook: %v", err)
	}

	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hook); err != nil {
		t.Fatalf("the hook is not in the hooks of the repository: %v", err)
	}
}

// A worktree has a git directory of its own, and the hooks of the
// repository live in the common directory: git reads them there.
func TestTheHookOfAWorktreeIsInstalledInTheRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatalf("make the repo: %v", err)
	}
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "-c", "user.email=doc@example.invalid", "-c", "user.name=doc", "commit", "-q", "--allow-empty", "-m", "init")

	worktree := filepath.Join(root, "worktree")
	runGit(t, repo, "worktree", "add", "-q", worktree, "-b", "worktree")
	t.Chdir(worktree)

	cmd := &InstallHookCmd{}
	if err := cmd.Run(nil, nil); err != nil {
		t.Fatalf("install the hook: %v", err)
	}

	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hook); err != nil {
		t.Fatalf("the hook is not in the hooks of the repository: %v", err)
	}
}

// runGit runs a git command in the given directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	//nolint:gosec // the command is built from constants
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
