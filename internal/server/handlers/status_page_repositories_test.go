package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/server/responses"
)

// The repositories of an explicit configuration are not in a discovery
// result: the status page showed no repository at all, and the operator
// saw "Repositories: 0".
func TestTheStatusPageShowsTheRepositoriesOfAnExplicitConfiguration(t *testing.T) {
	p := fakeStatusProvider{
		repoStatuses: []responses.RepositoryInfo{
			{URL: "https://example.com/docs.git", Name: "docs", Status: "ok"},
			{URL: "https://example.com/api.git", Name: "api", Status: "error"},
		},
	}

	repositories := generateRepositoryStatus(p)

	require.Len(t, repositories, 2)
	require.Equal(t, "docs", repositories[0].Name)
	require.Equal(t, "ok", repositories[0].Status)
	require.Equal(t, "error", repositories[1].Status)
}
