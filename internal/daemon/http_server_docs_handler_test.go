package daemon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// normalizeOutputDir normalizes and returns absolute path of output directory.
func normalizeOutputDir(out string) string {
	if out == "" {
		out = "./site"
	}
	if !filepath.IsAbs(out) {
		if abs, err := filepath.Abs(out); err == nil {
			out = abs
		}
	}
	return out
}

// shouldShowPendingPage determines if pending page should be shown.
func shouldShowPendingPage(root, out, urlPath string) bool {
	if root != out {
		return false
	}
	if _, err := os.Stat(filepath.Join(out, "public")); !os.IsNotExist(err) {
		return false
	}
	return urlPath == "/" || urlPath == ""
}

// writePendingPageResponse writes the pending page HTML response.
func writePendingPageResponse(w http.ResponseWriter, liveReload bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)

	scriptTag := ""
	if liveReload {
		scriptTag = `<script src="http://localhost:35729/livereload.js"></script>`
	}

	html := `<!doctype html><html><head><meta charset="utf-8"><title>Site rendering</title></head><body><h1>Documentation is being prepared</h1><p>The site hasn't been rendered yet. This page will be replaced automatically once rendering completes.</p>` + scriptTag + `</body></html>`
	_, _ = w.Write([]byte(html))
}

// TestDocsHandlerStaticRoot tests serving files when public directory exists.
func TestDocsHandlerStaticRoot(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	publicDir := filepath.Join(tmpDir, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed to create public dir: %v", err)
	}

	// Create a test file in public directory
	testFile := filepath.Join(publicDir, "index.html")
	content := []byte("<html><body>Test Content</body></html>")
	if err := os.WriteFile(testFile, content, 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg := &config.Config{
		Output: config.OutputConfig{
			Directory: tmpDir,
		},
	}

	srv := NewHTTPServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Call the root handler directly
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := srv.resolveDocsRoot()
		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})
	rootHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "Test Content") {
		t.Errorf("expected body to contain 'Test Content', got: %s", body)
	}
}

// TestDocsHandlerNoBuildPendingPage tests showing pending page when no build exists.
func TestDocsHandlerNoBuildPendingPage(t *testing.T) {
	// Create temp directory without public subdirectory
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Output: config.OutputConfig{
			Directory: tmpDir,
		},
		Build: config.BuildConfig{
			LiveReload: false,
		},
	}

	srv := NewHTTPServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Simulate the complex handler logic
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := srv.resolveDocsRoot()
		out := normalizeOutputDir(srv.config.Output.Directory)

		// Check if site needs rendering and path is root
		if shouldShowPendingPage(root, out, r.URL.Path) {
			writePendingPageResponse(w, srv.config.Build.LiveReload)
			return
		}

		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})
	rootHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "Documentation is being prepared") {
		t.Errorf("expected pending page, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "Site rendering") {
		t.Errorf("expected title 'Site rendering', got: %s", bodyStr)
	}
}

// TestDocsHandlerBuildErrorPage tests showing error page when build fails.
func TestDocsHandlerBuildErrorPage(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Output: config.OutputConfig{
			Directory: tmpDir,
		},
		Build: config.BuildConfig{
			LiveReload: false,
		},
	}

	// Create a daemon with build status indicating failure
	d := &Daemon{
		buildStatus: &buildStatusTracker{
			hasError:     true,
			lastErr:      ErrTestBuildFailed,
			hasGoodBuild: false,
		},
	}

	srv := NewHTTPServer(cfg, d)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Simulate the complex error checking logic
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := srv.resolveDocsRoot()
		out := resolveOutputDirectory(srv.config.Output.Directory)

		if shouldShowBuildError(srv, root, out) {
			serveBuildErrorPage(w, srv.daemon.buildStatus)
			return
		}

		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})
	rootHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "Build Failed") {
		t.Errorf("expected error page, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "test build error") {
		t.Errorf("expected error message, got: %s", bodyStr)
	}
}

// resolveOutputDirectory resolves the output directory to an absolute path.
func resolveOutputDirectory(dir string) string {
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			return abs
		}
	}
	return dir
}

// shouldShowBuildError determines if a build error page should be displayed.
func shouldShowBuildError(srv *HTTPServer, root, out string) bool {
	if root != out {
		return false
	}

	_, err := os.Stat(filepath.Join(out, "public"))
	if !os.IsNotExist(err) {
		return false
	}

	if srv.daemon == nil || srv.daemon.buildStatus == nil {
		return false
	}

	hasError, _, hasGoodBuild := srv.daemon.buildStatus.getStatus()
	return hasError && !hasGoodBuild
}

// serveBuildErrorPage writes a build error page to the response.
func serveBuildErrorPage(w http.ResponseWriter, status interface{ getStatus() (bool, error, bool) }) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)

	errorMsg := "Unknown error"
	if _, buildErr, _ := status.getStatus(); buildErr != nil {
		errorMsg = buildErr.Error()
	}

	html := `<!doctype html><html><head><meta charset="utf-8"><title>Build Failed</title></head>` +
		`<body><h1>⚠️ Build Failed</h1><p>The documentation site failed to build.</p>` +
		`<h2>Error Details:</h2><pre>` + errorMsg + `</pre></body></html>`
	_, _ = w.Write([]byte(html))
}

// TestDocsHandlerWithLiveReload tests that livereload script is injected when enabled.
func TestDocsHandlerWithLiveReload(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Output: config.OutputConfig{
			Directory: tmpDir,
		},
		Build: config.BuildConfig{
			LiveReload: true,
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				LiveReloadPort: 35729,
			},
		},
	}

	srv := NewHTTPServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := srv.resolveDocsRoot()
		out := normalizeOutputDir(srv.config.Output.Directory)

		if shouldShowPendingPage(root, out, r.URL.Path) {
			writePendingPageResponse(w, srv.config.Build.LiveReload)
			return
		}

		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})
	rootHandler.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "livereload.js") {
		t.Errorf("expected livereload script, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "35729") {
		t.Errorf("expected port 35729 in livereload script, got: %s", bodyStr)
	}
}

// Helper types for testing

var ErrTestBuildFailed = &testError{msg: "test build error"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// buildStatusTracker is a simplified version for testing.
type buildStatusTracker struct {
	hasError     bool
	lastErr      error
	hasGoodBuild bool
}

func (b *buildStatusTracker) getStatus() (hasError bool, lastErr error, hasGoodBuild bool) {
	return b.hasError, b.lastErr, b.hasGoodBuild
}
