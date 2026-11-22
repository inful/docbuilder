package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// newSiteTestServer serves the endpoints of a published site and records
// the queries it was asked.
func newSiteTestServer(t *testing.T, asked *http.Request) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		*asked = *r
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query": r.URL.Query().Get("q"),
			"results": []map[string]any{
				{"url": "/install/", "title": "Installing the daemon", "excerpt": "Install the daemon.", "score": 0.9},
			},
		})
	})
	mux.HandleFunc("/api/pages", func(w http.ResponseWriter, r *http.Request) {
		*asked = *r
		_ = json.NewEncoder(w).Encode(map[string]any{
			"pages": []map[string]any{{"url": "/install/", "title": "Installing the daemon"}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// siteRequest builds a call of a tool.
func siteRequest(name string, args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	return req
}

// siteContext carries the server state the handlers read.
func siteContext(t *testing.T, baseURL string) context.Context {
	t.Helper()
	state := &serverState{baseURL: baseURL, docsDir: ""}
	return context.WithValue(t.Context(), stateKey, state)
}

// toolText returns the text of a tool answer.
func toolText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if text, ok := result.Content[0].(mcp.TextContent); ok {
		return text.Text
	}
	return ""
}

// The search tool asks the published site and passes its answer on.
func TestSearchSiteAsksThePublishedSite(t *testing.T) {
	var asked http.Request
	srv := newSiteTestServer(t, &asked)

	result, err := handleSearchSite(siteContext(t, srv.URL),
		siteRequest("search_site", map[string]any{"query": "install", "limit": 3}))
	if err != nil {
		t.Fatalf("handleSearchSite: %v", err)
	}
	if result.IsError {
		t.Fatalf("the tool failed: %s", toolText(result))
	}
	if asked.URL.Query().Get("q") != "install" {
		t.Fatalf("the site was asked %q", asked.URL.RawQuery)
	}
	if asked.URL.Query().Get("limit") != "3" {
		t.Fatalf("the limit was not passed: %q", asked.URL.RawQuery)
	}
	if !strings.Contains(toolText(result), "/install/") {
		t.Fatalf("the answer holds no page: %s", toolText(result))
	}
}

// The pages tool asks the published site for the inventory.
func TestListSitePagesAsksThePublishedSite(t *testing.T) {
	var asked http.Request
	srv := newSiteTestServer(t, &asked)

	result, err := handleListSitePages(siteContext(t, srv.URL),
		siteRequest("list_site_pages", map[string]any{"prefix": "/install"}))
	if err != nil {
		t.Fatalf("handleListSitePages: %v", err)
	}
	if result.IsError {
		t.Fatalf("the tool failed: %s", toolText(result))
	}
	if asked.URL.Query().Get("prefix") != "/install" {
		t.Fatalf("the prefix was not passed: %q", asked.URL.RawQuery)
	}
	if !strings.Contains(toolText(result), "Installing the daemon") {
		t.Fatalf("the answer holds no page: %s", toolText(result))
	}
}

// A site that does not answer must not look like a site with no pages.
func TestSearchSiteReportsASiteThatDoesNotAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "the site keeps no index of its pages", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	result, err := handleSearchSite(siteContext(t, srv.URL),
		siteRequest("search_site", map[string]any{"query": "install"}))
	if err != nil {
		t.Fatalf("handleSearchSite: %v", err)
	}
	if !result.IsError {
		t.Fatalf("the tool did not fail: %s", toolText(result))
	}
	if !strings.Contains(toolText(result), "the site keeps no index of its pages") {
		t.Fatalf("the answer does not carry the reason: %s", toolText(result))
	}
}

// Without a site URL the tools say so instead of searching nothing.
func TestSearchSiteReportsAMissingSiteURL(t *testing.T) {
	result, err := handleSearchSite(siteContext(t, ""),
		siteRequest("search_site", map[string]any{"query": "install"}))
	if err != nil {
		t.Fatalf("handleSearchSite: %v", err)
	}
	if !result.IsError {
		t.Fatalf("the tool did not fail: %s", toolText(result))
	}
	if !strings.Contains(toolText(result), "no site URL configured") {
		t.Fatalf("the answer does not carry the reason: %s", toolText(result))
	}
}

// An empty query is refused: it would ask the site for nothing.
func TestSearchSiteRefusesAnEmptyQuery(t *testing.T) {
	result, err := handleSearchSite(siteContext(t, "http://127.0.0.1:1"),
		siteRequest("search_site", map[string]any{"query": "  "}))
	if err != nil {
		t.Fatalf("handleSearchSite: %v", err)
	}
	if !result.IsError {
		t.Fatalf("the tool did not fail: %s", toolText(result))
	}
}
