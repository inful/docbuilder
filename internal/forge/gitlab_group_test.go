package forge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The GitLab API needs the numeric id of a group: a group that the
// operator names by its path was not found, and the discovery of the
// whole forge failed.
func TestTheGitLabGroupIsResolvedToItsNumericID(t *testing.T) {
	var requested []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/groups":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 42, "name": "My Group", "path": "my-group", "full_path": "my-group", "full_name": "My Group", "kind": "group"},
			})
		case "/api/v4/groups/42/projects":
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
	if err != nil {
		t.Fatalf("create the client: %v", err)
	}

	repos, err := client.ListRepositories(context.Background(), []string{"my-group"})
	if err != nil {
		t.Fatalf("the repositories of the group were not found: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("the group has no repositories: %+v", repos)
	}

	found := false
	for _, path := range requested {
		if path == "/api/v4/groups/42/projects" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the group was not asked for with its numeric id: %v", requested)
	}
}

// A group that does not exist is an error with its name in it.
func TestTheGitLabGroupThatIsNotFoundIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v4/groups" {
			_ = json.NewEncoder(w).Encode([]map[string]any{})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	client, err := NewGitLabClient(&config.ForgeConfig{
		Name:    "gitlab",
		Type:    config.ForgeGitLab,
		APIURL:  srv.URL + "/api/v4",
		BaseURL: srv.URL,
		Auth:    &config.AuthConfig{Type: config.AuthTypeToken, Token: "token"},
	})
	if err != nil {
		t.Fatalf("create the client: %v", err)
	}

	if _, err := client.ListRepositories(context.Background(), []string{"nowhere"}); err == nil {
		t.Fatal("the group that does not exist was not an error")
	}
}
