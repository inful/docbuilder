package daemon

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	handlers "git.home.luguber.info/inful/docbuilder/internal/server/handlers"
	smw "git.home.luguber.info/inful/docbuilder/internal/server/middleware"
)

// parseHugoError extracts useful error information from Hugo build output.
// Hugo errors typically contain paths like: "/tmp/.../content/local/file.md:line:col": error message
// This function extracts: file.md:line:col: error message
func parseHugoError(errStr string) string {
	// Pattern 1: Match Hugo error format in output:
	// Error: error building site: process: readAndProcessContent: "/path/to/content/file.md:123:45": error message
	re1 := regexp.MustCompile(`Error:.*?[":]\s*"([^"]+\.md):(\d+):(\d+)":\s*(.+?)(?:\n|$)`)

	matches := re1.FindStringSubmatch(errStr)
	if len(matches) >= 5 {
		// Extract just the filename without full path
		filePath := matches[1]
		// Remove temporary directory prefix if present
		if idx := strings.Index(filePath, "/content/"); idx >= 0 {
			filePath = filePath[idx+9:] // Skip "/content/"
		}
		line := matches[2]
		col := matches[3]
		message := strings.TrimSpace(matches[4])
		return fmt.Sprintf("%s:%s:%s: %s", filePath, line, col, message)
	}

	// Pattern 2: Legacy format from previous implementation
	// "/path/to/content/local/relative/path.md:123:45": error message
	re2 := regexp.MustCompile(`/content/local/([^"]+):(\d+):(\d+)[^"]*":\s*(.+)$`)

	matches = re2.FindStringSubmatch(errStr)
	if len(matches) >= 5 {
		filePath := matches[1]
		line := matches[2]
		col := matches[3]
		message := strings.TrimSpace(matches[4])
		return fmt.Sprintf("%s:%s:%s: %s", filePath, line, col, message)
	}

	// If no pattern matches, return original error
	return errStr
}

// HTTPServer manages HTTP endpoints (docs, webhooks, admin) for the daemon.
type HTTPServer struct {
	docsServer    *http.Server
	webhookServer *http.Server
	adminServer   *http.Server
	config        *config.Config
	daemon        *Daemon // Reference to main daemon service
	errorAdapter  *derrors.HTTPErrorAdapter

	// Handler modules
	monitoringHandlers *handlers.MonitoringHandlers
	apiHandlers        *handlers.APIHandlers
	buildHandlers      *handlers.BuildHandlers
	webhookHandlers    *handlers.WebhookHandlers

	// middleware chain
	mchain func(http.Handler) http.Handler
}

// NewHTTPServer creates a new HTTP server instance with the specified configuration.
func NewHTTPServer(cfg *config.Config, daemon *Daemon) *HTTPServer {
	s := &HTTPServer{
		config:       cfg,
		daemon:       daemon,
		errorAdapter: derrors.NewHTTPErrorAdapter(slog.Default()),
	}

	// Create adapter for interfaces that need it
	adapter := &daemonAdapter{daemon: daemon}

	// Initialize handler modules
	s.monitoringHandlers = handlers.NewMonitoringHandlers(adapter)
	s.apiHandlers = handlers.NewAPIHandlers(cfg, adapter)
	s.buildHandlers = handlers.NewBuildHandlers(adapter)
	s.webhookHandlers = handlers.NewWebhookHandlers()

	// Initialize middleware chain
	s.mchain = smw.Chain(slog.Default(), s.errorAdapter)

	return s
}

// daemonAdapter adapts Daemon to handler interfaces
type daemonAdapter struct {
	daemon *Daemon
}

func (a *daemonAdapter) GetStatus() string {
	return string(a.daemon.GetStatus())
}

func (a *daemonAdapter) GetActiveJobs() int {
	return a.daemon.GetActiveJobs()
}

func (a *daemonAdapter) GetStartTime() time.Time {
	return a.daemon.GetStartTime()
}

// Metrics bridge for MonitoringHandlers
func (a *daemonAdapter) HTTPRequestsTotal() int {
	if a.daemon == nil || a.daemon.metrics == nil {
		return 0
	}
	snap := a.daemon.metrics.GetSnapshot()
	if v, ok := snap.Counters["http_requests_total"]; ok {
		return int(v)
	}
	return 0
}

func (a *daemonAdapter) RepositoriesTotal() int {
	if a.daemon == nil || a.daemon.metrics == nil {
		return 0
	}
	snap := a.daemon.metrics.GetSnapshot()
	if v, ok := snap.Gauges["repositories_discovered"]; ok {
		return int(v)
	}
	return 0
}

func (a *daemonAdapter) LastDiscoveryDurationSec() int {
	if a.daemon == nil || a.daemon.metrics == nil {
		return 0
	}
	snap := a.daemon.metrics.GetSnapshot()
	if h, ok := snap.Histograms["discovery_duration_seconds"]; ok {
		return int(h.Mean)
	}
	return 0
}

func (a *daemonAdapter) LastBuildDurationSec() int {
	if a.daemon == nil || a.daemon.metrics == nil {
		return 0
	}
	snap := a.daemon.metrics.GetSnapshot()
	if h, ok := snap.Histograms["build_duration_seconds"]; ok {
		return int(h.Mean)
	}
	return 0
}

func (a *daemonAdapter) TriggerDiscovery() string {
	return a.daemon.TriggerDiscovery()
}

func (a *daemonAdapter) TriggerBuild() string {
	return a.daemon.TriggerBuild()
}

func (a *daemonAdapter) GetQueueLength() int {
	return a.daemon.GetQueueLength()
}

// Start initializes and starts all HTTP servers.
func (s *HTTPServer) Start(ctx context.Context) error {
	if s.config.Daemon == nil {
		return fmt.Errorf("daemon configuration required for HTTP servers")
	}

	// Pre-bind all required ports so we can fail fast and surface aggregate errors instead of
	// logging three independent 'address already in use' lines after partial initialization.
	type preBind struct {
		name string
		port int
		ln   net.Listener
	}
	binds := []preBind{
		{name: "docs", port: s.config.Daemon.HTTP.DocsPort},
		{name: "webhook", port: s.config.Daemon.HTTP.WebhookPort},
		{name: "admin", port: s.config.Daemon.HTTP.AdminPort},
	}
	var bindErrs []error
	for i := range binds {
		addr := fmt.Sprintf(":%d", binds[i].port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			bindErrs = append(bindErrs, fmt.Errorf("%s port %d: %w", binds[i].name, binds[i].port, err))
			continue
		}
		binds[i].ln = ln
	}
	if len(bindErrs) > 0 {
		// Close any successful listeners before returning
		for _, b := range binds {
			if b.ln != nil {
				_ = b.ln.Close()
			}
		}
		return fmt.Errorf("http startup failed: %w", stdErrors.Join(bindErrs...))
	}

	// All ports bound successfully – now start servers handing them their pre-bound listeners.
	if err := s.startDocsServerWithListener(ctx, binds[0].ln); err != nil {
		return fmt.Errorf("failed to start docs server: %w", err)
	}
	if err := s.startWebhookServerWithListener(ctx, binds[1].ln); err != nil {
		return fmt.Errorf("failed to start webhook server: %w", err)
	}
	if err := s.startAdminServerWithListener(ctx, binds[2].ln); err != nil {
		return fmt.Errorf("failed to start admin server: %w", err)
	}

	slog.Info("HTTP servers started",
		slog.Int("docs_port", s.config.Daemon.HTTP.DocsPort),
		slog.Int("webhook_port", s.config.Daemon.HTTP.WebhookPort),
		slog.Int("admin_port", s.config.Daemon.HTTP.AdminPort))
	return nil
}

// Stop gracefully shuts down all HTTP servers.
func (s *HTTPServer) Stop(ctx context.Context) error {
	var errs []error

	// Stop servers in reverse order
	if s.adminServer != nil {
		if err := s.adminServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("admin server shutdown: %w", err))
		}
	}

	if s.webhookServer != nil {
		if err := s.webhookServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("webhook server shutdown: %w", err))
		}
	}

	if s.docsServer != nil {
		if err := s.docsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("docs server shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	slog.Info("HTTP servers stopped")
	return nil
}

// startDocsServerWithListener allows injecting a pre-bound listener (for coordinated bind checks).
// nolint:unparam // This method currently never returns an error.
func (s *HTTPServer) startDocsServerWithListener(_ context.Context, ln net.Listener) error {
	mux := http.NewServeMux()
	// Health/readiness endpoints on docs port as well for compatibility with common probe configs
	mux.HandleFunc("/health", s.monitoringHandlers.HandleHealthCheck)
	mux.HandleFunc("/healthz", s.monitoringHandlers.HandleHealthCheck) // Kubernetes-style alias
	mux.HandleFunc("/ready", s.handleReadiness)
	mux.HandleFunc("/readyz", s.handleReadiness) // Kubernetes-style alias

	// LiveReload endpoints (SSE + script) if enabled - MUST be before root handler
	if s.config.Build.LiveReload && s.daemon != nil && s.daemon.liveReload != nil {
		mux.Handle("/livereload", s.daemon.liveReload)
		mux.HandleFunc("/livereload.js", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			if _, err := w.Write([]byte(LiveReloadScript)); err != nil {
				slog.Error("failed to write livereload script", "error", err)
			}
		})
		slog.Info("LiveReload HTTP endpoints registered")
	}

	// Root handler dynamically chooses between the Hugo output directory and the rendered "public" folder.
	// This lets us begin serving immediately (before a static render completes) while automatically
	// switching to the fully rendered site once available—without restarting the daemon.
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := s.resolveDocsRoot()

		// If we're serving directly from the Hugo project (no public yet), avoid showing a raw directory listing.
		// Instead, return a brief 503 page indicating that a render is pending.
		out := s.config.Output.Directory
		if out == "" {
			out = "./site"
		}
		if !filepath.IsAbs(out) {
			if abs, err := filepath.Abs(out); err == nil {
				out = abs
			}
		}
		if root == out {
			if _, err := os.Stat(filepath.Join(out, "public")); os.IsNotExist(err) {
				// Check if there's a build error
				if s.daemon != nil && s.daemon.buildStatus != nil {
					if hasError, buildErr, hasGoodBuild := s.daemon.buildStatus.getStatus(); hasError && !hasGoodBuild {
						// Build failed and no previous successful build exists - show error page for all paths
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						w.WriteHeader(http.StatusServiceUnavailable)
						errorMsg := "Unknown error"
						if buildErr != nil {
							errorMsg = parseHugoError(buildErr.Error())
						}
						_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Build Failed</title><style>body{font-family:sans-serif;max-width:800px;margin:50px auto;padding:20px}h1{color:#d32f2f}pre{background:#f5f5f5;padding:15px;border-radius:4px;overflow-x:auto}</style></head><body><h1>⚠️ Build Failed</h1><p>The documentation site failed to build. Fix the error below and save to rebuild automatically.</p><h2>Error Details:</h2><pre>%s</pre><p><small>This page will refresh automatically when you fix the error.</small></p><script src="/livereload.js"></script></body></html>`, errorMsg)
						return
					}
				}
				// If requesting the root path, show a friendly pending page instead of a directory listing
				if r.URL.Path == "/" || r.URL.Path == "" {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>Site rendering</title></head><body><h1>Documentation is being prepared</h1><p>The site hasn't been rendered yet. This page will be replaced automatically once rendering completes.</p><script src="/livereload.js"></script></body></html>`))
					return
				}
			}
		}

		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})

	// Wrap with LiveReload injection middleware if enabled
	var rootWithMiddleware http.Handler = rootHandler
	if s.config.Build.LiveReload && s.daemon != nil && s.daemon.liveReload != nil {
		rootWithMiddleware = injectLiveReloadScript(rootHandler)
	}

	mux.Handle("/", s.mchain(rootWithMiddleware))

	// API endpoint for documentation status
	mux.HandleFunc("/api/status", s.apiHandlers.HandleDocsStatus)

	// Docs server needs long/no timeouts for SSE (LiveReload) connections
	// ReadTimeout: 0 = no timeout (SSE connections are long-lived)
	// WriteTimeout: 0 = no timeout (SSE connections send periodic pings)
	// IdleTimeout: still set to close truly idle connections
	s.docsServer = &http.Server{Handler: mux, ReadTimeout: 0, WriteTimeout: 0, IdleTimeout: 300 * time.Second}
	return s.startServerWithListener("docs", s.docsServer, ln)
}

// resolveDocsRoot picks the directory to serve. Preference order:
// 1. <outputDir>/public if it exists (Hugo static render completed)
// 2. <outputDir> (Hugo project scaffold / in-progress)
func (s *HTTPServer) resolveDocsRoot() string {
	out := s.config.Output.Directory
	if out == "" {
		out = "./site"
	}
	// Normalize to absolute path once; failures just return original path
	if !filepath.IsAbs(out) {
		if abs, err := filepath.Abs(out); err == nil {
			out = abs
		}
	}

	// First, try the public directory (fully rendered site)
	public := filepath.Join(out, "public")
	if st, err := os.Stat(public); err == nil && st.IsDir() {
		return public
	}

	// If public doesn't exist, check if we're in the middle of a rebuild
	// and the previous backup directory exists
	prev := out + "_prev"
	prevPublic := filepath.Join(prev, "public")
	if st, err := os.Stat(prevPublic); err == nil && st.IsDir() {
		// Serve from previous backup to avoid empty responses during atomic rename
		return prevPublic
	}

	return out
}

// nolint:unparam // This method currently never returns an error.
func (s *HTTPServer) startWebhookServerWithListener(_ context.Context, ln net.Listener) error {
	mux := http.NewServeMux()

	// Webhook endpoints for each forge type
	mux.HandleFunc("/webhooks/github", s.webhookHandlers.HandleGitHubWebhook)
	mux.HandleFunc("/webhooks/gitlab", s.webhookHandlers.HandleGitLabWebhook)
	mux.HandleFunc("/webhooks/forgejo", s.webhookHandlers.HandleForgejoWebhook)

	// Generic webhook endpoint (auto-detects forge type)
	mux.HandleFunc("/webhook", s.webhookHandlers.HandleGenericWebhook)

	s.webhookServer = &http.Server{Handler: s.mchain(mux), ReadTimeout: 30 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	return s.startServerWithListener("webhook", s.webhookServer, ln)
}

// handleReadiness returns 200 only when the rendered static site exists under <output.directory>/public.
// Otherwise it returns 503 to signal not-yet-ready (e.g., first build pending or failed).
func (s *HTTPServer) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("method not allowed"))
		return
	}
	out := s.config.Output.Directory
	if out == "" {
		out = "./site"
	}
	if !filepath.IsAbs(out) {
		if abs, err := filepath.Abs(out); err == nil {
			out = abs
		}
	}
	public := filepath.Join(out, "public")
	if st, err := os.Stat(public); err == nil && st.IsDir() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("not ready: public directory missing"))
}

// nolint:unparam // This method currently never returns an error.
func (s *HTTPServer) startAdminServerWithListener(_ context.Context, ln net.Listener) error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc(s.config.Monitoring.Health.Path, s.monitoringHandlers.HandleHealthCheck)
	mux.HandleFunc("/healthz", s.monitoringHandlers.HandleHealthCheck) // Kubernetes-style alias
	// Readiness endpoint: only ready when a rendered site exists under <output>/public
	mux.HandleFunc("/ready", s.handleReadiness)
	mux.HandleFunc("/readyz", s.handleReadiness) // Kubernetes-style alias
	// Add enhanced health check endpoint (if daemon is available)
	if s.daemon != nil {
		mux.HandleFunc("/health/detailed", s.daemon.EnhancedHealthHandler)
	} else {
		// Fallback for refactored daemon
		mux.HandleFunc("/health/detailed", s.monitoringHandlers.HandleHealthCheck)
	}

	// Metrics endpoint
	if s.config.Monitoring.Metrics.Enabled {
		mux.HandleFunc(s.config.Monitoring.Metrics.Path, s.monitoringHandlers.HandleMetrics)
		// Add detailed metrics endpoint (if daemon is available)
		if s.daemon != nil && s.daemon.metrics != nil {
			mux.HandleFunc("/metrics/detailed", s.daemon.metrics.MetricsHandler)
		} else {
			// Fallback for refactored daemon
			mux.HandleFunc("/metrics/detailed", s.monitoringHandlers.HandleMetrics)
		}
		if h := prometheusOptionalHandler(); h != nil {
			mux.Handle("/metrics/prometheus", h)
		}
	}

	// Administrative endpoints
	mux.HandleFunc("/api/daemon/status", s.apiHandlers.HandleDaemonStatus)
	mux.HandleFunc("/api/daemon/config", s.apiHandlers.HandleDaemonConfig)
	mux.HandleFunc("/api/discovery/trigger", s.buildHandlers.HandleTriggerDiscovery)
	mux.HandleFunc("/api/build/trigger", s.buildHandlers.HandleTriggerBuild)
	mux.HandleFunc("/api/build/status", s.buildHandlers.HandleBuildStatus)
	mux.HandleFunc("/api/repositories", s.buildHandlers.HandleRepositories)

	// Status page endpoint (HTML and JSON)
	mux.HandleFunc("/status", s.daemon.StatusHandler)

	s.adminServer = &http.Server{Handler: s.mchain(mux), ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	return s.startServerWithListener("admin", s.adminServer, ln)
}

// startServerWithListener launches an http.Server on a pre-bound listener or binds itself.
// It standardizes goroutine startup and error logging across server types.
func (s *HTTPServer) startServerWithListener(kind string, srv *http.Server, ln net.Listener) error {
	go func() {
		var err error
		if ln != nil {
			err = srv.Serve(ln)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error(fmt.Sprintf("%s server error", kind), "error", err)
		}
	}()
	return nil
}

// prometheusOptionalHandler returns the Prometheus metrics handler. Previously
// this was gated behind a build tag; it now always returns a handler.

// inline middleware removed in favor of internal/server/middleware

// injectLiveReloadScript is a middleware that injects the LiveReload client script
// into HTML responses.
func injectLiveReloadScript(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only inject into HTML pages (not assets, API endpoints, etc.)
		path := r.URL.Path
		isHTMLPage := path == "/" || path == "" || strings.HasSuffix(path, "/") || strings.HasSuffix(path, ".html")

		if !isHTMLPage {
			// Not an HTML page, serve normally
			next.ServeHTTP(w, r)
			return
		}

		injector := newLiveReloadInjector(w, r)
		next.ServeHTTP(injector, r)
		injector.finalize()
	})
}

// liveReloadInjector wraps an http.ResponseWriter to inject the LiveReload client script
// into HTML responses before </body> tag. Uses buffering with a size limit to prevent stalls.
type liveReloadInjector struct {
	http.ResponseWriter
	statusCode    int
	buffer        []byte
	headerWritten bool
	passthrough   bool
	maxSize       int
}

func newLiveReloadInjector(w http.ResponseWriter, _ *http.Request) *liveReloadInjector {
	return &liveReloadInjector{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		maxSize:        512 * 1024, // 512KB max - typical HTML page
	}
}

func (l *liveReloadInjector) WriteHeader(code int) {
	l.statusCode = code
	// Don't write header yet unless in passthrough mode
	if l.passthrough {
		l.ResponseWriter.WriteHeader(code)
		l.headerWritten = true
	}
}

func (l *liveReloadInjector) Write(data []byte) (int, error) {
	// Check Content-Type on first write
	if !l.headerWritten && !l.passthrough && l.buffer == nil {
		contentType := l.ResponseWriter.Header().Get("Content-Type")
		isHTML := contentType == "" || strings.Contains(contentType, "text/html")

		if !isHTML {
			// Not HTML - passthrough
			l.passthrough = true
			l.ResponseWriter.WriteHeader(l.statusCode)
			l.headerWritten = true
			return l.ResponseWriter.Write(data)
		}

		l.buffer = make([]byte, 0, 64*1024) // Start with 64KB
	}

	if l.passthrough {
		return l.ResponseWriter.Write(data)
	}

	// Check if buffering would exceed limit
	if len(l.buffer)+len(data) > l.maxSize {
		// Too large - switch to passthrough, flush buffer, write remaining
		l.passthrough = true
		l.ResponseWriter.Header().Del("Content-Length")
		l.ResponseWriter.WriteHeader(l.statusCode)
		l.headerWritten = true

		if len(l.buffer) > 0 {
			if _, err := l.ResponseWriter.Write(l.buffer); err != nil {
				return 0, err
			}
		}
		return l.ResponseWriter.Write(data)
	}

	// Buffer the data
	l.buffer = append(l.buffer, data...)
	return len(data), nil
}

// finalize must be called after the handler completes to inject the script
func (l *liveReloadInjector) finalize() {
	if l.passthrough || len(l.buffer) == 0 {
		if !l.headerWritten {
			l.ResponseWriter.WriteHeader(l.statusCode)
		}
		return
	}

	// Inject script before </body>
	html := string(l.buffer)
	script := `<script src="/livereload.js"></script></body>`
	modified := strings.Replace(html, "</body>", script, 1)

	l.ResponseWriter.Header().Del("Content-Length")
	l.ResponseWriter.WriteHeader(l.statusCode)
	_, _ = l.ResponseWriter.Write([]byte(modified))
}
