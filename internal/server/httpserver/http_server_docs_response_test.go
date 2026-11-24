package httpserver

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The docs server serves the site to the public. Every response used to
// be held in memory until it had been written, so the daemon's memory
// was "the biggest file on the site times the number of readers at
// once". Only a LiveReload request needs its response captured, so
// every other request has to go straight to the file server.
func TestDocsServer_ServesTheBodyWithoutCapturingIt(t *testing.T) {
	tmpDir := t.TempDir()
	publicDir := filepath.Join(tmpDir, "public")
	if err := os.MkdirAll(filepath.Join(publicDir, "guide"), 0o750); err != nil {
		t.Fatalf("create public dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "index.html"), []byte("<html>Home</html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "guide", "index.html"), []byte("<html>Guide</html>"), 0o600); err != nil {
		t.Fatalf("write guide/index.html: %v", err)
	}

	docsPort := freePort(t)
	cfg := &config.Config{
		Monitoring: &config.MonitoringConfig{
			Health: config.MonitoringHealth{Path: "/health"},
		},
		Output: config.OutputConfig{Directory: tmpDir},
		Build:  config.BuildConfig{LiveReload: true},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				DocsPort:    docsPort,
				WebhookPort: freePort(t),
				AdminPort:   freePort(t),
				AdminBind:   "127.0.0.1",
			},
		},
	}

	srv := New(cfg, testRuntime{}, Options{})
	if err := srv.Start(t.Context()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	base := "http://127.0.0.1:" + strconv.Itoa(docsPort)

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// A page that exists is served with its body.
	resp, err := client.Do(mustRequest(t, base+"/"))
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !strings.Contains(string(body), "Home") {
		t.Fatalf("GET / body = %q, want it to contain %q", body, "Home")
	}

	// A page that does not exist is a 404, not a redirect: the redirect
	// is only for LiveReload.
	resp, err = client.Do(mustRequest(t, base+"/guide/missing"))
	if err != nil {
		t.Fatalf("get missing page: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /guide/missing = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	// The LiveReload request still gets redirected to the nearest page
	// that exists.
	req := mustRequest(t, base+"/guide/missing")
	req.AddCookie(&http.Cookie{
		Name:     "docbuilder_lr_reload",
		Value:    "1",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("get missing page with cookie: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("GET /guide/missing with the LiveReload cookie = %d, want %d", resp.StatusCode, http.StatusTemporaryRedirect)
	}
	if got := resp.Header.Get("Location"); got != "/guide/" {
		t.Fatalf("Location = %q, want %q", got, "/guide/")
	}
}

// mustRequest builds a GET request for the test.
func mustRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	return req
}
