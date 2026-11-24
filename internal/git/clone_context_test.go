package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// localRemote creates a bare repository with one commit and returns its path.
func localRemote(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	bare := filepath.Join(tmp, "remote.git")
	if _, err := git.PlainInit(bare, true); err != nil {
		t.Fatalf("init bare: %v", err)
	}
	work := filepath.Join(tmp, "work")
	repo, err := git.PlainInit(work, false)
	if err != nil {
		t.Fatalf("init work: %v", err)
	}
	if writeErr := os.WriteFile(filepath.Join(work, "a.txt"), []byte("a"), 0o600); writeErr != nil {
		t.Fatalf("write file: %v", writeErr)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Add("a.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("a", &git.CommitOptions{Author: &object.Signature{Name: "a", Email: "a@b"}}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := repo.CreateRemote(&ggitcfg.RemoteConfig{Name: "origin", URLs: []string{bare}}); err != nil {
		t.Fatalf("remote: %v", err)
	}
	if err := repo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil {
		t.Fatalf("push: %v", err)
	}
	return bare
}

// A clone that is canceled does not clone. The clone took no context
// before: a host that never answers kept the build worker busy until
// the operating system gave up, and the build could not be canceled.
func TestTheCloneOfACanceledBuildDoesNotClone(t *testing.T) {
	bare := localRemote(t)
	workspace := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(workspace)
	repo := appcfg.Repository{Name: "repo", URL: bare, Branch: "master"}

	if _, err := client.CloneRepoWithMetadata(ctx, repo); err == nil {
		t.Fatal("the clone of a canceled build succeeded")
	}

	if _, err := os.Stat(filepath.Join(workspace, "repo")); err == nil {
		t.Fatal("the clone of a canceled build left a directory behind")
	}
}

// A canceled build does not retry either: the work it was doing is not
// wanted any more.
func TestACanceledBuildDoesNotRetryTheGitOperation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(t.TempDir()).WithBuildConfig(&appcfg.BuildConfig{MaxRetries: 3})

	calls := 0
	_, err := client.withRetry(ctx, "clone", "repo", func() (string, error) {
		calls++
		return "", nil
	})
	if err == nil {
		t.Fatal("the retry of a canceled build succeeded")
	}
	if calls != 0 {
		t.Fatalf("the canceled build tried %d times", calls)
	}
}
