package stages

import (
	"context"
	"errors"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// The state of the repositories, as the build writes it.
type fakeErrorState struct {
	errors map[string]string
}

func (f *fakeErrorState) SetRepoDocumentCount(string, int)     {}
func (f *fakeErrorState) SetRepoDocFilesHash(string, string)   {}
func (f *fakeErrorState) SetRepoLastError(url, message string) { f.errors[url] = message }
func (f *fakeErrorState) EnsureRepositoryState(url, name, branch string) {
	if _, known := f.errors[url]; !known {
		f.errors[url] = ""
	}
}

// The generator of a build that writes the state of the repositories.
type fakeErrorGenerator struct {
	state *fakeErrorState
}

func (f *fakeErrorGenerator) Config() *config.Config     { return &config.Config{} }
func (f *fakeErrorGenerator) OutputDir() string          { return "" }
func (f *fakeErrorGenerator) StageDir() string           { return "" }
func (f *fakeErrorGenerator) Recorder() metrics.Recorder { return nil }
func (f *fakeErrorGenerator) StateManager() state.RepositoryMetadataWriter {
	return f.state
}
func (f *fakeErrorGenerator) ComputeConfigHash() string  { return "" }
func (f *fakeErrorGenerator) GenerateHugoConfig() error  { return nil }
func (f *fakeErrorGenerator) BuildRoot() string          { return "" }
func (f *fakeErrorGenerator) CreateHugoStructure() error { return nil }
func (f *fakeErrorGenerator) CopyContentFilesWithState(context.Context, []docs.DocFile, *models.BuildState) error {
	return nil
}
func (f *fakeErrorGenerator) Observer() models.BuildObserver { return nil }
func (f *fakeErrorGenerator) ExistingSiteValidForSkip() bool { return false }
func (f *fakeErrorGenerator) SkipIfUnchanged() bool          { return false }
func (f *fakeErrorGenerator) Renderer() models.Renderer      { return nil }

// The error of a repository that could not be cloned is written to the
// state.
//
// The error was never written: the status of a repository whose clone
// failed was "ok", because the counter of the last successful build was
// not zero.
func TestTheErrorOfAFailedCloneIsWrittenToTheState(t *testing.T) {
	fakeState := &fakeErrorState{errors: map[string]string{}}
	bs := models.NewBuildState(&fakeErrorGenerator{state: fakeState}, nil, models.NewBuildReport(context.Background(), 0, 0))

	repo := config.Repository{Name: "repoA", URL: "https://example.com/repoA.git"}
	recordCloneFailure(bs, repo, RepoFetchResult{Name: "repoA", Err: errors.New("the clone failed")})

	if got := fakeState.errors[repo.URL]; got != "the clone failed" {
		t.Fatalf("the error of the repository is %q", got)
	}

	// The next build succeeds: the error is taken away.
	bs.Git.RepoPaths = map[string]string{}
	recordCloneSuccess(bs, repo, RepoFetchResult{Name: "repoA", Path: "/tmp/repoA"})

	if got := fakeState.errors[repo.URL]; got != "" {
		t.Fatalf("the error of the repository is still %q", got)
	}
}
