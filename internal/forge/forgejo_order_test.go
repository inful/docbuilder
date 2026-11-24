package forge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The order of the repositories has to be the order they were found
// in. A map has no order, so the same repository changed its name - and
// its place in the site - from one run to the next.
func TestTheForgejoRepositoriesKeepTheirOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user/orgs":
			_, _ = w.Write([]byte(`[{"id":1,"username":"orgB"}]`))
		case "/user/repos":
			_, _ = w.Write([]byte(`[{"full_name":"orgA/docs","name":"docs","owner":{"login":"orgA"}}]`))
		case "/orgs/orgB/repos":
			_, _ = w.Write([]byte(`[{"full_name":"orgB/docs","name":"docs","owner":{"login":"orgB"}}]`))
		default:
			_ = json.NewEncoder(w).Encode([]any{})
		}
	}))
	defer srv.Close()

	client, err := NewForgejoClient(&Config{
		Name:    "forgejo",
		Type:    config.ForgeForgejo,
		APIURL:  srv.URL,
		BaseURL: srv.URL,
		Auth:    &config.AuthConfig{Type: config.AuthTypeToken, Token: "token"},
	})
	require.NoError(t, err)

	first, err := client.ListRepositories(t.Context(), []string{"orgB"})
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.Equal(t, "orgA/docs", first[0].FullName)
	require.Equal(t, "orgB/docs", first[1].FullName)

	// The order is the same every time.
	for range 20 {
		again, listErr := client.ListRepositories(t.Context(), []string{"orgB"})
		require.NoError(t, listErr)
		require.Len(t, again, 2)
		require.Equal(t, first[0].FullName, again[0].FullName)
		require.Equal(t, first[1].FullName, again[1].FullName)
	}
}
