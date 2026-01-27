package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

type testRuntime struct{}

func (testRuntime) GetStatus() string                                  { return "" }
func (testRuntime) GetActiveJobs() int                                 { return 0 }
func (testRuntime) GetStartTime() time.Time                            { return time.Time{} }
func (testRuntime) HTTPRequestsTotal() int                             { return 0 }
func (testRuntime) RepositoriesTotal() int                             { return 0 }
func (testRuntime) LastDiscoveryDurationSec() int                      { return 0 }
func (testRuntime) LastBuildDurationSec() int                          { return 0 }
func (testRuntime) TriggerDiscovery() string                           { return "" }
func (testRuntime) TriggerBuild() string                               { return "" }
func (testRuntime) TriggerWebhookBuild(_, _ string, _ []string) string { return "" }
func (testRuntime) GetQueueLength() int                                { return 0 }

type testBuildStatus struct {
	hasError     bool
	err          error
	hasGoodBuild bool
}

func (b testBuildStatus) GetStatus() (bool, error, bool) {
	return b.hasError, b.err, b.hasGoodBuild
}

type testLiveReloadHub struct{}

func (testLiveReloadHub) ServeHTTP(http.ResponseWriter, *http.Request) {}
func (testLiveReloadHub) Broadcast(string)                             {}
func (testLiveReloadHub) Shutdown()                                    {}

// TestDocsHandlerStaticRoot tests serving files when public directory exists.
func TestDocsHandlerStaticRoot(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	publicDir := filepath.Join(tmpDir, "public")
	if err := os.MkdirAll(publicDir, 0o750); err != nil {
		t.Fatalf("failed to create public dir: %v", err)
	}

	// Create a test file in public directory
	testFile := filepath.Join(publicDir, "index.html")
	content := []byte("<html><body>Test Content</body></html>")
	if err := os.WriteFile(testFile, content, 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg := &config.Config{
		Output: config.OutputConfig{
			Directory: tmpDir,
		},
	}

	srv := New(cfg, testRuntime{}, Options{})

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

	srv := New(cfg, testRuntime{}, Options{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	root := srv.resolveDocsRoot()
	if !srv.shouldShowStatusPage(root) {
		t.Fatalf("expected shouldShowStatusPage=true")
	}
	srv.handleStatusPage(rec, req, root)

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

	srv := New(cfg, testRuntime{}, Options{BuildStatus: testBuildStatus{hasError: true, err: ErrTestBuildFailed, hasGoodBuild: false}})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	root := srv.resolveDocsRoot()
	if !srv.shouldShowStatusPage(root) {
		t.Fatalf("expected shouldShowStatusPage=true")
	}
	srv.handleStatusPage(rec, req, root)

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

	srv := New(cfg, testRuntime{}, Options{LiveReloadHub: testLiveReloadHub{}})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	root := srv.resolveDocsRoot()
	if !srv.shouldShowStatusPage(root) {
		t.Fatalf("expected shouldShowStatusPage=true")
	}
	srv.handleStatusPage(rec, req, root)

	body, _ := io.ReadAll(rec.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "livereload.js") {
		t.Errorf("expected livereload script, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "35729") {
		t.Errorf("expected port 35729 in livereload script, got: %s", bodyStr)
	}
}

var ErrTestBuildFailed = &testError{msg: "test build error"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
