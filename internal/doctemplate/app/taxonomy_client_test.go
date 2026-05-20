package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchTaxonomies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/taxonomies.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"tags":["alpha","Beta","alpha"],"categories":["Guides","guides"]}`))
	}))
	defer srv.Close()

	tags, categories, err := fetchTaxonomies(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("fetchTaxonomies failed: %v", err)
	}
	if len(tags) != 2 || tags[0] != "alpha" || tags[1] != "Beta" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	if len(categories) != 1 || categories[0] != "Guides" {
		t.Fatalf("unexpected categories: %#v", categories)
	}
}
