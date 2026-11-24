package httpserver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A big file has to reach the browser whole, also when the request
// carries the LiveReload cookie. The answer used to be held in memory
// until it was written, so a 20 MiB file held 104 MiB and every reader
// at once held its own copy.
func TestTheDocsServerServesABigFileWholeWithTheLiveReloadCookie(t *testing.T) {
	tmpDir := t.TempDir()
	publicDir := filepath.Join(tmpDir, "public")
	if err := os.MkdirAll(publicDir, 0o750); err != nil {
		t.Fatalf("create public dir: %v", err)
	}
	content := bytes.Repeat([]byte("a"), 2<<20)
	if err := os.WriteFile(filepath.Join(publicDir, "big.bin"), content, 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
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

	req := mustRequest(t, base+"/big.bin")
	req.AddCookie(&http.Cookie{
		Name:     "docbuilder_lr_reload",
		Value:    "1",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get the file: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the file was answered with %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read the file: %v", err)
	}
	if len(body) != len(content) {
		t.Fatalf("the client got %d bytes, want %d", len(body), len(content))
	}
}
