package httpserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The docs server must serve the pages of the site at /api/pages, the
// same way it serves the search at /api/search.
func TestDocsServerServesThePages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := &config.Config{}

	pages := map[string]any{"pages": []map[string]string{{"url": "/install/", "title": "Installing the daemon"}}}
	srv := New(cfg, testRuntime{}, Options{
		PagesHandle: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(pages)
		},
		SearchHandle: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"query": "", "results": []any{}})
		},
	})

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	if err := srv.startDocsServerWithListener(ctx, listener); err != nil {
		t.Fatalf("start the docs server: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, path := range []string{"/api/pages", "/api/search"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+listener.Addr().String()+path, nil)
		if err != nil {
			t.Fatalf("build the request for %s: %v", path, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			t.Fatalf("get %s: status = %d", path, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}
}
