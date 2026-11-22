package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/search"
)

// The endpoint answers with the pages the index holds.
func TestSearchServiceListsThePages(t *testing.T) {
	service := newTestSearchService(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/pages", nil)
	rec := httptest.NewRecorder()
	service.HandlePages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Pages []search.Page `json:"pages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("read the response: %v", err)
	}
	if len(response.Pages) != 2 {
		t.Fatalf("got %d pages, want 2: %+v", len(response.Pages), response.Pages)
	}
	if response.Pages[0].URL != "/deploy/kubernetes/" {
		t.Fatalf("first page = %+v", response.Pages[0])
	}
	if response.Pages[0].Title != "Deploying to Kubernetes" {
		t.Fatalf("title = %q", response.Pages[0].Title)
	}
}

// A prefix keeps only the pages whose address starts with it.
func TestSearchServiceListsThePrefixedPages(t *testing.T) {
	service := newTestSearchService(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/pages?prefix=/install", nil)
	rec := httptest.NewRecorder()
	service.HandlePages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Pages []search.Page `json:"pages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("read the response: %v", err)
	}
	if len(response.Pages) != 1 {
		t.Fatalf("got %d pages, want 1: %+v", len(response.Pages), response.Pages)
	}
	if response.Pages[0].URL != "/install/" {
		t.Fatalf("page = %+v", response.Pages[0])
	}
}

// A site that keeps no index answers with an error, not with an empty
// list: an empty list would read as "the site has no pages".
func TestSearchServiceListsNoPagesWithoutAnIndex(t *testing.T) {
	var service *SearchService

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/pages", nil)
	rec := httptest.NewRecorder()
	service.HandlePages(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
