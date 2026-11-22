package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// A repository that is versioned by a tag must survive the second
// build. The update path fetches a branch, and a tag is not one: the
// fetch failed and the pages of the tag vanished from the site.
func TestUpdateOfATagKeepsTheTag(t *testing.T) {
	origin := t.TempDir()
	repo, err := git.PlainInit(origin, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	addSimpleCommit(t, repo, origin, "a.txt")

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if _, tagErr := repo.CreateTag("v1.2.3", head.Hash(), nil); tagErr != nil {
		t.Fatalf("tag: %v", tagErr)
	}

	workspace := t.TempDir()
	client := NewClient(workspace)

	tagged := appcfg.Repository{
		Name:   "tagged",
		URL:    origin,
		Branch: "v1.2.3",
		IsTag:  true,
	}

	if _, cloneErr := client.CloneRepo(tagged); cloneErr != nil {
		t.Fatalf("clone the tag: %v", cloneErr)
	}

	// The second build takes the update path.
	if _, updateErr := client.UpdateRepo(tagged); updateErr != nil {
		t.Fatalf("update the tag: %v", updateErr)
	}

	clonePath := filepath.Join(workspace, "tagged")
	if _, statErr := os.Stat(filepath.Join(clonePath, "a.txt")); statErr != nil {
		t.Fatalf("the clone of the tag is gone: %v", statErr)
	}

	// The clone is at the commit of the tag.
	ref, err := repo.Tag("v1.2.3")
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
	tagHash := ref.Hash()
	if tagHash != head.Hash() {
		t.Fatalf("the tag points at %s, want %s", tagHash, head.Hash())
	}
}
