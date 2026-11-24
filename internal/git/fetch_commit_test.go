package git

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// A commit that is not the tip of the branch is not in a shallow
// clone. The daemon pins the commit of the webhook, and a push that
// landed while the build ran is such a commit: the build must not fail
// over it.
func TestFetchCommitFetchesACommitThatIsNotTheTip(t *testing.T) {
	tmp := t.TempDir()
	barePath := filepath.Join(tmp, "remote.git")
	if _, err := git.PlainInit(barePath, true); err != nil {
		t.Fatalf("init bare: %v", err)
	}

	workPath := filepath.Join(tmp, "seed")
	workRepo, wErr := git.PlainInit(workPath, false)
	if wErr != nil {
		t.Fatalf("init work: %v", wErr)
	}
	if _, cErr := workRepo.CreateRemote(&ggitcfg.RemoteConfig{Name: "origin", URLs: []string{barePath}}); cErr != nil {
		t.Fatalf("remote: %v", cErr)
	}

	var firstCommit plumbing.Hash
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if aErr := addFileAndCommit(workRepo, workPath, name, name, name); aErr != nil {
			t.Fatalf("commit %s: %v", name, aErr)
		}
		head, hErr := workRepo.Head()
		if hErr != nil {
			t.Fatalf("head: %v", hErr)
		}
		if firstCommit.IsZero() {
			firstCommit = head.Hash()
		}
	}
	if pErr := workRepo.Push(&git.PushOptions{RemoteName: "origin"}); pErr != nil {
		t.Fatalf("push: %v", pErr)
	}

	// A shallow clone of the tip: the first commit is not in it.
	localPath := filepath.Join(tmp, "local")
	if _, cErr := git.PlainClone(localPath, false, &git.CloneOptions{
		URL:           barePath,
		ReferenceName: plumbing.NewBranchReferenceName("master"),
		SingleBranch:  true,
		Depth:         1,
	}); cErr != nil {
		t.Fatalf("clone: %v", cErr)
	}

	local, oErr := git.PlainOpen(localPath)
	if oErr != nil {
		t.Fatalf("open local: %v", oErr)
	}
	if _, sErr := local.CommitObject(firstCommit); sErr == nil {
		t.Fatal("the shallow clone has the first commit: the test does not prove anything")
	}

	client := NewClient(t.TempDir())
	repo := appcfg.Repository{Name: "repo", URL: barePath, Branch: "master"}
	if err := client.FetchCommit(context.Background(), localPath, repo, firstCommit.String()); err != nil {
		t.Fatalf("fetch the commit: %v", err)
	}

	// The handle of the repository was opened before the fetch: it does
	// not see the objects that the fetch brought.
	local, oErr = git.PlainOpen(localPath)
	if oErr != nil {
		t.Fatalf("open local: %v", oErr)
	}
	if _, err := local.CommitObject(firstCommit); err != nil {
		t.Fatalf("the commit was not fetched: %v", err)
	}
}
