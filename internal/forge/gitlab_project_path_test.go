package forge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The path of a project of GitLab is "group%2Fproject", and the URL
// escapes it again when it is written: the request that left the daemon
// was "/api/v4/projects/group%252Fproject", and GitLab answered 404.
// A repository that gains documentation never came back into the site
// through a webhook.
func TestThePathOfTheProjectIsEscapedOnce(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 1, "name": "project", "path_with_namespace": "group/project"}`))
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

	_, err = client.GetRepository(t.Context(), "group", "project")
	require.NoError(t, err)

	require.Equal(t, "/api/v4/projects/group%2Fproject", seen,
		"the path of the project was escaped twice")
}
