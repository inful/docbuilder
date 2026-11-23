package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// A history that cannot be read is not a divergence. With
// hard_reset_on_diverge - on by default - the branch is aligned to the
// remote instead of failing the build. The error the update returned
// before even asked for the setting that was already on.
func TestAnUnreadableHistoryIsAlignedToTheRemoteWhenHardResetIsOn(t *testing.T) {
	tmp := t.TempDir()
	barePath := filepath.Join(tmp, "remote.git")
	if _, err := git.PlainInit(barePath, true); err != nil {
		t.Fatalf("init bare: %v", err)
	}

	workPath := filepath.Join(tmp, "seed")
	workRepo, err := git.PlainInit(workPath, false)
	if err != nil {
		t.Fatalf("init work: %v", err)
	}
	if _, remoteErr := workRepo.CreateRemote(&ggitcfg.RemoteConfig{Name: "origin", URLs: []string{barePath}}); remoteErr != nil {
		t.Fatalf("create remote: %v", remoteErr)
	}
	for _, name := range []string{"a", "b"} {
		if commitErr := addFileAndCommit(workRepo, workPath, name+".txt", name, name); commitErr != nil {
			t.Fatalf("commit %s: %v", name, commitErr)
		}
	}
	if pushErr := workRepo.Push(&git.PushOptions{RemoteName: "origin"}); pushErr != nil {
		t.Fatalf("push: %v", pushErr)
	}

	// The clone is shallow, exactly like the daemon clones with
	// shallow_depth: 1: the history of the first commit is not on disk.
	localPath := filepath.Join(tmp, "local")
	if _, cloneErr := git.PlainClone(localPath, false, &git.CloneOptions{
		URL:           barePath,
		ReferenceName: plumbing.NewBranchReferenceName("master"),
		SingleBranch:  true,
		Depth:         1,
	}); cloneErr != nil {
		t.Fatalf("clone local: %v", cloneErr)
	}

	local, err := git.PlainOpen(localPath)
	if err != nil {
		t.Fatalf("open local: %v", err)
	}
	wt, err := local.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	head, err := local.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}

	// The remote is gone: the fetch that would deepen the history fails.
	if err := os.Rename(barePath, barePath+".gone"); err != nil {
		t.Fatalf("hide the remote: %v", err)
	}

	// The local branch points at the first commit, which the shallow
	// clone does not have: the history cannot be read.
	localRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName("master"), plumbing.NewHash("1111111111111111111111111111111111111111"))
	remoteRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName("master"), head.Hash())

	repoCfg := appcfg.Repository{Name: "repo", URL: barePath, Branch: "master"}

	hard := NewClient(t.TempDir()).WithBuildConfig(&appcfg.BuildConfig{HardResetOnDiverge: true})
	if err := hard.syncWithRemote(local, wt, repoCfg, "master", localRef, remoteRef); err != nil {
		t.Fatalf("the branch was not aligned to the remote: %v", err)
	}

	// Without the setting the update fails, as it should.
	strict := NewClient(t.TempDir()).WithBuildConfig(&appcfg.BuildConfig{HardResetOnDiverge: false})
	if err := strict.syncWithRemote(local, wt, repoCfg, "master", localRef, remoteRef); err == nil {
		t.Fatal("the update did not fail without hard_reset_on_diverge")
	}
}
