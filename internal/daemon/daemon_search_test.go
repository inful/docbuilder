package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/search"
)

// newTestSearchService returns a search service with a small index.
func newTestSearchService(t *testing.T) *SearchService {
	t.Helper()
	index, err := search.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { _ = index.Close() })

	if err := index.Upsert(context.Background(), []search.Document{
		{URL: "/install/", Title: "Installing the daemon", Body: "Install the daemon with the package manager."},
		{URL: "/deploy/kubernetes/", Title: "Deploying to Kubernetes", Body: "Deploy the daemon to a cluster."},
	}); err != nil {
		t.Fatalf("index: %v", err)
	}
	return &SearchService{index: index, maxResults: 10}
}

// The endpoint answers a query with the pages that match it.
func TestSearchServiceHandlesAQuery(t *testing.T) {
	service := newTestSearchService(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/search?q=package", nil)
	rec := httptest.NewRecorder()
	service.HandleSearch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var response struct {
		Query   string          `json:"query"`
		Results []search.Result `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("read the response: %v", err)
	}
	if response.Query != "package" {
		t.Fatalf("query = %q", response.Query)
	}
	if len(response.Results) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(response.Results), response.Results)
	}
	if response.Results[0].URL != "/install/" {
		t.Fatalf("result = %+v", response.Results[0])
	}
	if response.Results[0].Title != "Installing the daemon" {
		t.Fatalf("title = %q", response.Results[0].Title)
	}
}

// An empty query is answered with no results, not with an error.
func TestSearchServiceHandlesAnEmptyQuery(t *testing.T) {
	service := newTestSearchService(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/search", nil)
	rec := httptest.NewRecorder()
	service.HandleSearch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var response struct {
		Results []search.Result `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("read the response: %v", err)
	}
	if len(response.Results) != 0 {
		t.Fatalf("got %d results, want none", len(response.Results))
	}
	// The answer holds a list of results even when there are none: a
	// reader of the answer should not have to care about the difference
	// between an empty list and nothing.
	if !strings.Contains(rec.Body.String(), `"results":[]`) {
		t.Fatalf("the answer does not hold an empty list: %s", rec.Body.String())
	}
}
