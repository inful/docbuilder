package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/search"
)

// The visitor may ask for fewer results than the configuration allows.
// The limit of the configuration is the ceiling.
func TestSearchServiceHonoursTheLimit(t *testing.T) {
	service := newTestSearchService(t)

	// Three pages match "the".
	if err := service.index.Upsert(context.Background(), []search.Document{
		{URL: "/a/", Title: "A", Body: "the page"},
		{URL: "/b/", Title: "B", Body: "the page"},
		{URL: "/c/", Title: "C", Body: "the page"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	cases := []struct {
		name string
		url  string
		want int
	}{
		{"no limit", "/api/search?q=the", 3},
		{"a limit of one", "/api/search?q=the&limit=1", 1},
		{"a limit of two", "/api/search?q=the&limit=2", 2},
		{"a limit above the ceiling", "/api/search?q=the&limit=100", 3},
		{"a limit of zero", "/api/search?q=the&limit=0", 3},
		{"a limit that is not a number", "/api/search?q=the&limit=lots", 3},
		{"a negative limit", "/api/search?q=the&limit=-1", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.url, nil)
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
			if len(response.Results) != tc.want {
				t.Fatalf("got %d results, want %d", len(response.Results), tc.want)
			}
		})
	}
}
