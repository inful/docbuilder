package models

import "testing"

// A repository that was added to the configuration has no previous
// head. The site does not hold its pages yet, so the build must run.
func TestAllReposUnchangedIsFalseForANewRepository(t *testing.T) {
	gs := &GitState{
		PreHeads:  map[string]string{"repoA": "aaa"},
		PostHeads: map[string]string{"repoA": "aaa", "repoB": "bbb"},
	}
	if gs.AllReposUnchangedComputed() {
		t.Fatal("a new repository must not count as unchanged")
	}
}

// Nothing changed: the build may be skipped.
func TestAllReposUnchangedIsTrueWhenNothingMoved(t *testing.T) {
	gs := &GitState{
		PreHeads:  map[string]string{"repoA": "aaa", "repoB": "bbb"},
		PostHeads: map[string]string{"repoA": "aaa", "repoB": "bbb"},
	}
	if !gs.AllReposUnchangedComputed() {
		t.Fatal("nothing moved, so the repositories are unchanged")
	}
}

// A head that moved: the build must run.
func TestAllReposUnchangedIsFalseWhenAHeadMoved(t *testing.T) {
	gs := &GitState{
		PreHeads:  map[string]string{"repoA": "aaa"},
		PostHeads: map[string]string{"repoA": "ccc"},
	}
	if gs.AllReposUnchangedComputed() {
		t.Fatal("a moved head must not count as unchanged")
	}
}
