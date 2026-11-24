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

// TestShallowCloneFastForwardsAfterSeveralCommits proves that a shallow
// clone still fast-forwards when the remote moved more than one commit.
// The history needed for the ancestry question is not on disk in that case,
// and before the fix the update was reported as a divergence.
func TestShallowCloneFastForwardsAfterSeveralCommits(t *testing.T) {
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
	if commitErr := addFileAndCommit(workRepo, workPath, "a.txt", "A", "A"); commitErr != nil {
		t.Fatalf("commit A: %v", commitErr)
	}
	if pushErr := workRepo.Push(&git.PushOptions{RemoteName: "origin"}); pushErr != nil {
		t.Fatalf("push A: %v", pushErr)
	}

	// Clone shallow, exactly like the daemon does with shallow_depth: 1.
	localPath := filepath.Join(tmp, "local")
	if _, cloneErr := git.PlainClone(localPath, false, &git.CloneOptions{
		URL:           barePath,
		ReferenceName: plumbing.NewBranchReferenceName("master"),
		SingleBranch:  true,
		Depth:         1,
	}); cloneErr != nil {
		t.Fatalf("clone local: %v", cloneErr)
	}

	// The remote moves three commits ahead.
	for _, name := range []string{"b", "c", "d"} {
		if commitErr := addFileAndCommit(workRepo, workPath, name+".txt", name, name); commitErr != nil {
			t.Fatalf("commit %s: %v", name, commitErr)
		}
	}
	if pushErr := workRepo.Push(&git.PushOptions{RemoteName: "origin"}); pushErr != nil {
		t.Fatalf("push bcd: %v", pushErr)
	}

	repoCfg := appcfg.Repository{Name: "repo", URL: barePath, Branch: "master"}
	client := NewClient(t.TempDir()).WithBuildConfig(&appcfg.BuildConfig{
		ShallowDepth:       1,
		HardResetOnDiverge: false,
	})
	if _, updateErr := client.updateExistingRepo(t.Context(), localPath, repoCfg); updateErr != nil {
		t.Fatalf("expected fast-forward, got %v", updateErr)
	}

	updated, err := git.PlainOpen(localPath)
	if err != nil {
		t.Fatalf("open updated: %v", err)
	}
	head, err := updated.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	remote, err := updated.Reference(plumbing.NewRemoteReferenceName("origin", "master"), true)
	if err != nil {
		t.Fatalf("remote ref: %v", err)
	}
	if head.Hash() != remote.Hash() {
		t.Fatalf("expected head %s to equal remote %s", head.Hash(), remote.Hash())
	}
	if _, statErr := os.Stat(filepath.Join(localPath, "d.txt")); statErr != nil {
		t.Fatalf("expected the new file in the working tree: %v", statErr)
	}
}
