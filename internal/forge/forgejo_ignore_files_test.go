package forge

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The files that keep a repository out of the site are the ones of the
// configuration. The field was never read: every forge looked for
// ".docignore", so "ignore_files: [.myignore]" kept excluding
// ".docignore" and never excluded ".myignore".
func TestTheConfiguredIgnoreFileKeepsTheRepositoryOutOfTheSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, ".myignore") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("[]"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client, err := NewForgejoClient(&Config{
		Name:        "forgejo",
		Type:        config.ForgeForgejo,
		APIURL:      srv.URL,
		BaseURL:     srv.URL,
		IgnoreFiles: []string{".myignore"},
		Auth:        &config.AuthConfig{Type: config.AuthTypeToken, Token: "token"},
	})
	require.NoError(t, err)

	repo := &Repository{FullName: "org/repo", DefaultBranch: "main"}
	require.NoError(t, client.CheckDocumentation(t.Context(), repo))
	require.True(t, repo.HasDocIgnore, "the repository with the configured ignore file is in the site")

	// Without the configured file the repository is not excluded.
	plain, err := NewForgejoClient(&Config{
		Name:    "forgejo",
		Type:    config.ForgeForgejo,
		APIURL:  srv.URL,
		BaseURL: srv.URL,
		Auth:    &config.AuthConfig{Type: config.AuthTypeToken, Token: "token"},
	})
	require.NoError(t, err)

	other := &Repository{FullName: "org/repo", DefaultBranch: "main"}
	require.NoError(t, plain.CheckDocumentation(t.Context(), other))
	require.False(t, other.HasDocIgnore, "the repository without an ignore file is out of the site")
}
