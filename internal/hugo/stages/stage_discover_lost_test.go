package stages

import (
	"testing"
)

// fakeCountReader answers the number of documents of a repository.
type fakeCountReader struct {
	counts map[string]int
}

func (f *fakeCountReader) GetRepoDocumentCount(url string) int { return f.counts[url] }

// A repository that had documentation and has none now has lost it:
// the directory was renamed, or the files were deleted. The build has to
// fail: the site that is published now would have the pages of that
// repository missing, and it would replace the site that still has them.
func TestPersistDiscoveredDocsReportsALostRepository(t *testing.T) {
	reader := &fakeCountReader{counts: map[string]int{"https://example.com/repoA.git": 5}}

	if !repositoryLostDocumentation(reader, "https://example.com/repoA.git", 0) {
		t.Fatal("the build did not report the repository that lost its documentation")
	}
}

// A repository that never had documentation is filtered, and the build
// goes on: a repository without documentation is not an error.
func TestPersistDiscoveredDocsAcceptsARepositoryWithoutDocuments(t *testing.T) {
	reader := &fakeCountReader{counts: map[string]int{}}

	if repositoryLostDocumentation(reader, "https://example.com/repoA.git", 0) {
		t.Fatal("the build refused a repository that never had documentation")
	}
}
