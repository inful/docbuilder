package stages

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// The state of the commits of the repositories.
type fakeCommitState struct {
	commits map[string]string
}

func (f *fakeCommitState) SetRepoDocumentCount(string, int)    {}
func (f *fakeCommitState) SetRepoDocFilesHash(string, string)  {}
func (f *fakeCommitState) SetRepoLastError(string, string)     {}
func (f *fakeCommitState) GetRepoLastCommit(url string) string { return f.commits[url] }

// The site that is published was built from the commits of the
// workspace.
//
// The heads of the workspace alone do not say it: a build that failed
// after the clone stage leaves the commits of the next build in the
// workspace, and the site is still the one of the build before it. The
// next build saw "no HEAD changes" and skipped, and the content never
// reached the site.
func TestThePublishedCommitsMatchTheWorkspace(t *testing.T) {
	state := &fakeCommitState{commits: map[string]string{
		"https://example.com/repoA.git": "old-commit",
	}}

	bs := models.NewBuildState(&fakeErrorGenerator{state: state}, nil, models.NewBuildReport(t.Context(), 0, 0))
	bs.Git.Repositories = []config.Repository{{Name: "repoA", URL: "https://example.com/repoA.git"}}
	bs.Git.PostHeads = map[string]string{"repoA": "new-commit"}

	if publishedCommitsMatchTheWorkspace(bs) {
		t.Fatal("the workspace is ahead of the published site, and the build would skip")
	}

	// The site was published from the commits of the workspace.
	state.commits["https://example.com/repoA.git"] = "new-commit"
	if !publishedCommitsMatchTheWorkspace(bs) {
		t.Fatal("the published site was built from the commits of the workspace")
	}
}
