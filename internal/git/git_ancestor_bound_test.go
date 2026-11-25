package git

import (
	"fmt"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

// The walk that looks for the ancestor reads the commits of the history
// one by one, and a branch that does not lead to the commit reads the
// whole history: a repository with a hundred thousand commits took
// seconds for every update.
func TestTheWalkForTheAncestorIsBounded(t *testing.T) {
	tmp := t.TempDir()
	repo, err := git.PlainInit(tmp, false)
	require.NoError(t, err)

	// A history of twenty commits.
	commits := make([]plumbing.Hash, 0, 20)
	for i := range 20 {
		commits = append(commits, addCommit(t, repo, tmp, fmt.Sprintf("file-%d.txt", i), "content", "commit"))
	}

	// The walk is bounded: the answer comes at once, and the branch is
	// moved to the remote.
	ancestor, err := isAncestorWithin(repo, commits[0], commits[len(commits)-1], 5)
	require.NoError(t, err)
	require.False(t, ancestor, "the walk did not stop at the bound")

	// The commit that is in the history is found.
	ancestor, err = isAncestorWithin(repo, commits[0], commits[len(commits)-1], 100)
	require.NoError(t, err)
	require.True(t, ancestor, "the commit of the history was not found")
}
