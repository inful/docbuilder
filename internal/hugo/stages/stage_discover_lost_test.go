package stages

import (
	"testing"
)

// fakeCountReader answers the number of documents a repository had in the
// last build.
type fakeCountReader struct {
	counts map[string]int
}

func (f *fakeCountReader) GetRepoDocumentCount(url string) int { return f.counts[url] }

// A repository that had documentation and has none now has lost it. The
// repositories that have no file are not in the list of the files, and
// the guard has to look at the repositories of the build.
func TestARepositoryWithoutFilesIsComparedWithTheState(t *testing.T) {
	reader := &fakeCountReader{counts: map[string]int{"https://example.com/repoA.git": 5}}

	if !repositoryLostDocumentation(reader, "https://example.com/repoA.git", 0) {
		t.Fatal("the repository that lost its documentation was not reported")
	}

	// A repository that has files is not reported.
	if repositoryLostDocumentation(reader, "https://example.com/repoA.git", 3) {
		t.Fatal("a repository with documents was reported")
	}
}
