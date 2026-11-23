package git

import (
	"testing"
)

// The hash of the remote can be empty.
//
// The remote has no branch of that name, and the slice of the empty
// hash stopped the daemon.
func TestTheLogOfASkippedFetchSurvivesAnEmptyHash(t *testing.T) {
	// The call must not panic.
	logSkippedFetch("repo", "main", "")
	logSkippedFetch("repo", "main", "abc")
}
