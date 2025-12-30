package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// helper to create a commit (duplicate of logic in other tests but kept local to avoid accidental coupling).
func addSimpleCommit(t *testing.T, repo *git.Repository, repoPath, name string) plumbing.Hash {
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	file := filepath.Join(repoPath, name)
	if writeErr := os.WriteFile(file, []byte(name), 0o600); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	if _, addErr := wt.Add(name); addErr != nil {
		t.Fatalf("add: %v", addErr)
	}
	h, err := wt.Commit(name, &git.CommitOptions{Author: &object.Signature{Name: "tester", Email: "t@example.com", When: time.Now()}})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return h
}

func TestResolveTargetBranchExplicit(t *testing.T) {
	tmp := t.TempDir()
	repo, err := git.PlainInit(tmp, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	addSimpleCommit(t, repo, tmp, "a.txt")
	cfg := appcfg.Repository{Name: "r", URL: "https://example/repo.git", Branch: "feature-x"}
	// explicit branch should always win
	b, err := resolveTargetBranch(repo, cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if b != "feature-x" {
		t.Fatalf("expected feature-x got %s", b)
	}
}

func TestResolveTargetBranchFromHead(t *testing.T) {
	tmp := t.TempDir()
	repo, err := git.PlainInit(tmp, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	addSimpleCommit(t, repo, tmp, "a.txt")
	cfg := appcfg.Repository{Name: "r", URL: "https://example/repo.git"}
	b, err := resolveTargetBranch(repo, cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// initial branch name depends on git implementation (master/main). Accept either.
	if b != "master" && b != "main" {
		t.Fatalf("expected master or main got %s", b)
	}
}

func TestUpdateExistingRepoFastForward(t *testing.T) {
	tmp := t.TempDir()
	bare := filepath.Join(tmp, "remote.git")
	if _, err := git.PlainInit(bare, true); err != nil {
		t.Fatalf("init bare: %v", err)
	}
	// seed working with commit A then push
	seedPath := filepath.Join(tmp, "seed")
	seedRepo, err := git.PlainInit(seedPath, false)
	if err != nil {
		t.Fatalf("init seed: %v", err)
	}
	if _, err := seedRepo.CreateRemote(&ggitcfg.RemoteConfig{Name: "origin", URLs: []string{bare}}); err != nil {
		t.Fatalf("remote: %v", err)
	}
	addSimpleCommit(t, seedRepo, seedPath, "a.txt")
	if err := seedRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil {
		t.Fatalf("push A: %v", err)
	}

	// clone local to update later
	localPath := filepath.Join(tmp, "local")
	if _, err := git.PlainClone(localPath, false, &git.CloneOptions{URL: bare}); err != nil {
		t.Fatalf("clone: %v", err)
	}
	// add commit B to remote (via seed) and push
	addSimpleCommit(t, seedRepo, seedPath, "b.txt")
	if err := seedRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil {
		t.Fatalf("push B: %v", err)
	}

	// record remote hash for master/main
	seedHead, _ := seedRepo.Head()
	remoteHash := seedHead.Hash()

	repoCfg := appcfg.Repository{Name: "repo", URL: bare}
	client := NewClient(tmp)
	if _, err := client.updateExistingRepo(localPath, repoCfg); err != nil {
		t.Fatalf("updateExistingRepo fast-forward: %v", err)
	}
	updated, _ := git.PlainOpen(localPath)
	head, _ := updated.Head()
	if head.Hash() != remoteHash {
		t.Fatalf("expected fast-forward head %s remote %s", head.Hash(), remoteHash)
	}
}
