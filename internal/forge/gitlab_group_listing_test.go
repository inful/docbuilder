package forge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The groups are listed once: the listing is paginated, and a listing
// per group burns the rate limit that the listing of the repositories
// needs.
func TestTheGitLabGroupsAreListedOnce(t *testing.T) {
	listings := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/groups":
			listings++
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 41, "name": "First", "path": "first", "full_path": "first", "kind": "group"},
				{"id": 42, "name": "Second", "path": "second", "full_path": "second", "kind": "group"},
				{"id": 43, "name": "Third", "path": "third", "full_path": "third", "kind": "group"},
			})
		case "/api/v4/groups/41/projects", "/api/v4/groups/42/projects", "/api/v4/groups/43/projects":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := NewGitLabClient(&config.ForgeConfig{
		Name:    "gitlab",
		Type:    config.ForgeGitLab,
		APIURL:  srv.URL + "/api/v4",
		BaseURL: srv.URL,
		Auth:    &config.AuthConfig{Type: config.AuthTypeToken, Token: "token"},
	})
	require.NoError(t, err)

	_, err = client.ListRepositories(t.Context(), []string{"first", "second", "third"})
	require.NoError(t, err)

	require.Equal(t, 1, listings, "the groups were listed once per group")
}
