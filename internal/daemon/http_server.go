package daemon

import (
	"context"
	"errors"
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
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	handlers "git.home.luguber.info/inful/docbuilder/internal/server/handlers"
	smw "git.home.luguber.info/inful/docbuilder/internal/server/middleware"
)

// parseHugoError extracts useful error information from Hugo build output.
// Hugo errors typically contain paths like: "/tmp/.../content/local/file.md:line:col": error message
// This function extracts: file.md:line:col: error message.
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
	docsServer       *http.Server
	webhookServer    *http.Server
	adminServer      *http.Server
	liveReloadServer *http.Server
	config           *config.Config
	daemon           *Daemon // Reference to main daemon service
	errorAdapter     *derrors.HTTPErrorAdapter

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

	// Extract webhook configs and forge clients for webhook handlers
	webhookConfigs := make(map[string]*config.WebhookConfig)
	forgeClients := make(map[string]forge.Client)
	if daemon != nil && daemon.forgeManager != nil {
		for forgeName := range daemon.forgeManager.GetAllForges() {
			client := daemon.forgeManager.GetForge(forgeName)
			if client != nil {
				forgeClients[forgeName] = client
			}
		}
	}
	for _, forge := range cfg.Forges {
		if forge.Webhook != nil {
			webhookConfigs[forge.Name] = forge.Webhook
		}
	}

	s.webhookHandlers = handlers.NewWebhookHandlers(adapter, forgeClients, webhookConfigs)

	// Initialize middleware chain
	s.mchain = smw.Chain(slog.Default(), s.errorAdapter)

	return s
}

// daemonAdapter adapts Daemon to handler interfaces.
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

// Metrics bridge for MonitoringHandlers.
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

func (a *daemonAdapter) TriggerWebhookBuild(repoFullName, branch string) string {
	return a.daemon.TriggerWebhookBuild(repoFullName, branch)
}

func (a *daemonAdapter) GetQueueLength() int {
	return a.daemon.GetQueueLength()
}

// Start initializes and starts all HTTP servers.
func (s *HTTPServer) Start(ctx context.Context) error {
	if s.config.Daemon == nil {
		return errors.New("daemon configuration required for HTTP servers")
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
	// Add LiveReload port if LiveReload is enabled
	if s.config.Build.LiveReload && s.daemon != nil && s.daemon.liveReload != nil {
		binds = append(binds, preBind{name: "livereload", port: s.config.Daemon.HTTP.LiveReloadPort})
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
		return fmt.Errorf("http startup failed: %w", errors.Join(bindErrs...))
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

	// Start LiveReload server if enabled
	if s.config.Build.LiveReload && s.daemon != nil && s.daemon.liveReload != nil && len(binds) > 3 {
		if err := s.startLiveReloadServerWithListener(ctx, binds[3].ln); err != nil {
			return fmt.Errorf("failed to start livereload server: %w", err)
		}
		slog.Info("HTTP servers started",
			slog.Int("docs_port", s.config.Daemon.HTTP.DocsPort),
			slog.Int("webhook_port", s.config.Daemon.HTTP.WebhookPort),
			slog.Int("admin_port", s.config.Daemon.HTTP.AdminPort),
			slog.Int("livereload_port", s.config.Daemon.HTTP.LiveReloadPort))
	} else {
		slog.Info("HTTP servers started",
			slog.Int("docs_port", s.config.Daemon.HTTP.DocsPort),
			slog.Int("webhook_port", s.config.Daemon.HTTP.WebhookPort),
			slog.Int("admin_port", s.config.Daemon.HTTP.AdminPort))
	}
	return nil
}

// Stop gracefully shuts down all HTTP servers.
func (s *HTTPServer) Stop(ctx context.Context) error {
	var errs []error

	// Stop servers in reverse order
	if s.liveReloadServer != nil {
		if err := s.liveReloadServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("livereload server shutdown: %w", err))
		}
	}

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
						scriptTag := ""
						if s.config.Build.LiveReload {
							scriptTag = fmt.Sprintf(`<script src="http://localhost:%d/livereload.js"></script>`, s.config.Daemon.HTTP.LiveReloadPort)
						}
						_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Build Failed</title><style>body{font-family:sans-serif;max-width:800px;margin:50px auto;padding:20px}h1{color:#d32f2f}pre{background:#f5f5f5;padding:15px;border-radius:4px;overflow-x:auto}</style></head><body><h1>⚠️ Build Failed</h1><p>The documentation site failed to build. Fix the error below and save to rebuild automatically.</p><h2>Error Details:</h2><pre>%s</pre><p><small>This page will refresh automatically when you fix the error.</small></p>%s</body></html>`, errorMsg, scriptTag)
						return
					}
				}
				// If requesting the root path, show a friendly pending page instead of a directory listing
				if r.URL.Path == "/" || r.URL.Path == "" {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusServiceUnavailable)
					scriptTag := ""
					if s.config.Build.LiveReload {
						scriptTag = fmt.Sprintf(`<script src="http://localhost:%d/livereload.js"></script>`, s.config.Daemon.HTTP.LiveReloadPort)
					}
					_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Site rendering</title></head><body><h1>Documentation is being prepared</h1><p>The site hasn't been rendered yet. This page will be replaced automatically once rendering completes.</p>%s</body></html>`, scriptTag)
					return
				}
			}
		}

		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})

	// Wrap with 404 fallback that redirects to nearest parent path on LiveReload
	rootWithFallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the response to detect 404s
		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		rootHandler.ServeHTTP(rec, r)

		// If we got a 404 and this is a GET request from LiveReload, try to redirect to parent
		if rec.statusCode == http.StatusNotFound && r.Method == http.MethodGet {
			// Check if this is a LiveReload-triggered reload via Cookie
			if cookie, err := r.Cookie("docbuilder_lr_reload"); err == nil && cookie.Value == "1" {
				root := s.resolveDocsRoot()
				redirectPath := s.findNearestValidParent(root, r.URL.Path)
				if redirectPath != "" && redirectPath != r.URL.Path {
					// Clear the cookie and redirect
					http.SetCookie(w, &http.Cookie{
						Name:   "docbuilder_lr_reload",
						Value:  "",
						MaxAge: -1,
						Path:   "/",
					})
					w.Header().Set("Location", redirectPath)
					w.WriteHeader(http.StatusTemporaryRedirect)
					return
				}
			}
		}

		// If not redirecting, flush the captured response
		rec.Flush()
	})

	// Wrap with Cache-Control headers for static assets
	rootWithCaching := s.addCacheControlHeaders(rootWithFallback)

	// Wrap with LiveReload injection middleware if enabled
	rootWithMiddleware := rootWithCaching
	if s.config.Build.LiveReload && s.daemon != nil && s.daemon.liveReload != nil {
		rootWithMiddleware = s.injectLiveReloadScriptWithPort(rootWithCaching, s.config.Daemon.HTTP.LiveReloadPort)
	}

	mux.Handle("/", s.mchain(rootWithMiddleware))

	// API endpoint for documentation status
	mux.HandleFunc("/api/status", s.apiHandlers.HandleDocsStatus)

	// Docs server now uses standard timeouts since SSE moved to separate port
	s.docsServer = &http.Server{Handler: mux, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	return s.startServerWithListener("docs", s.docsServer, ln)
}

// resolveDocsRoot picks the directory to serve. Preference order:
// 1. <outputDir>/public if it exists (Hugo static render completed)
// 2. <outputDir> (Hugo project scaffold / in-progress).
func (s *HTTPServer) resolveDocsRoot() string {
	out := s.config.Output.Directory
	if out == "" {
		out = "./site"
	}
	// Combine with base_directory if set and path is relative
	if s.config.Output.BaseDirectory != "" && !filepath.IsAbs(out) {
		out = filepath.Join(s.config.Output.BaseDirectory, out)
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
		slog.Debug("Serving from primary public directory",
			slog.String("path", public),
			slog.Time("modified", st.ModTime()))
		return public
	}

	// If public doesn't exist, check if we're in the middle of a rebuild
	// and the previous backup directory exists
	prev := out + "_prev"
	prevPublic := filepath.Join(prev, "public")
	if st, err := os.Stat(prevPublic); err == nil && st.IsDir() {
		// Serve from previous backup to avoid empty responses during atomic rename
		slog.Warn("Serving from backup directory - primary public missing",
			slog.String("backup_path", prevPublic),
			slog.String("expected_path", public),
			slog.Time("backup_modified", st.ModTime()))
		return prevPublic
	}

	slog.Warn("No public directory found, serving from output root",
		slog.String("path", out),
		slog.String("expected_public", public),
		slog.String("expected_backup", prevPublic))
	return out
}

// findNearestValidParent walks up the URL path hierarchy to find the nearest existing page.
func (s *HTTPServer) findNearestValidParent(root, urlPath string) string {
	// Clean the path
	urlPath = filepath.Clean(urlPath)

	// Try parent paths, working upward
	for urlPath != "/" && urlPath != "." {
		urlPath = filepath.Dir(urlPath)
		if urlPath == "." {
			urlPath = "/"
		}

		// Check if this path exists as index.html
		testPath := filepath.Join(root, urlPath, "index.html")
		if _, err := os.Stat(testPath); err == nil {
			// Ensure path ends with / for directory-style URLs
			if !strings.HasSuffix(urlPath, "/") {
				urlPath += "/"
			}
			return urlPath
		}

		// Also check direct file
		if urlPath != "/" {
			testPath = filepath.Join(root, urlPath)
			if stat, err := os.Stat(testPath); err == nil && !stat.IsDir() {
				return urlPath
			}
		}
	}

	// Fall back to root
	return "/"
}

// addCacheControlHeaders wraps a handler to add appropriate Cache-Control headers for static assets.
// Different asset types receive different cache durations:
// - Immutable assets (CSS, JS, fonts, images): 1 year (31536000s)
// - HTML pages: no cache (to ensure content updates are immediately visible)
// - Other assets: short cache (5 minutes).
func (s *HTTPServer) addCacheControlHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Determine cache policy based on file extension
		if strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") {
			// CSS and JavaScript - cache for 1 year (Hugo typically uses content hashing)
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasSuffix(path, ".woff") || strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".ttf") || strings.HasSuffix(path, ".eot") ||
			strings.HasSuffix(path, ".otf") {
			// Web fonts - cache for 1 year
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") ||
			strings.HasSuffix(path, ".jpeg") || strings.HasSuffix(path, ".gif") ||
			strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".webp") ||
			strings.HasSuffix(path, ".ico") {
			// Images - cache for 1 week
			w.Header().Set("Cache-Control", "public, max-age=604800")
		} else if strings.HasSuffix(path, ".pdf") || strings.HasSuffix(path, ".zip") ||
			strings.HasSuffix(path, ".tar") || strings.HasSuffix(path, ".gz") {
			// Downloadable files - cache for 1 day
			w.Header().Set("Cache-Control", "public, max-age=86400")
		} else if strings.HasSuffix(path, ".json") && !strings.Contains(path, "search") {
			// JSON data files (except search indices) - cache for 5 minutes
			w.Header().Set("Cache-Control", "public, max-age=300")
		} else if strings.HasSuffix(path, ".xml") {
			// XML files (RSS, sitemaps) - cache for 1 hour
			w.Header().Set("Cache-Control", "public, max-age=3600")
		} else if strings.HasSuffix(path, ".html") || path == "/" || !strings.Contains(path, ".") {
			// HTML pages and directories - no cache to ensure content updates are visible
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		}
		// For all other files, don't set Cache-Control (let browser use default behavior)

		next.ServeHTTP(w, r)
	})
}

// responseRecorder captures the status code and body from the underlying handler.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
	body       []byte
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.written = true
	}
	// Don't write to underlying writer yet - we might redirect
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}

func (r *responseRecorder) Flush() {
	if r.written {
		r.ResponseWriter.WriteHeader(r.statusCode)
	}
	if len(r.body) > 0 {
		_, _ = r.ResponseWriter.Write(r.body)
	}
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
	// Combine with base_directory if set and path is relative
	if s.config.Output.BaseDirectory != "" && !filepath.IsAbs(out) {
		out = filepath.Join(s.config.Output.BaseDirectory, out)
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

// startLiveReloadServerWithListener starts the dedicated LiveReload SSE server.
// nolint:unparam // This method currently never returns an error.
func (s *HTTPServer) startLiveReloadServerWithListener(_ context.Context, ln net.Listener) error {
	mux := http.NewServeMux()

	// CORS middleware for LiveReload server (allows cross-origin requests from docs port)
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	// LiveReload SSE endpoint
	if s.daemon != nil && s.daemon.liveReload != nil {
		mux.Handle("/livereload", corsMiddleware(s.daemon.liveReload))
		mux.HandleFunc("/livereload.js", func(w http.ResponseWriter, r *http.Request) {
			// Add CORS headers for script loading
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			// Generate script that connects to this dedicated port
			script := fmt.Sprintf(`(() => {
  if (window.__DOCBUILDER_LR__) return;
  window.__DOCBUILDER_LR__=true;
  function connect(){
    const es = new EventSource('http://localhost:%d/livereload');
    let first=true; let current=null;
    es.onmessage = (e)=>{ try { const p=JSON.parse(e.data); if(first){ current=p.hash; first=false; return;} if(p.hash && p.hash!==current){ console.log('[docbuilder] change detected, reloading'); document.cookie='docbuilder_lr_reload=1; path=/; max-age=5'; location.reload(); } } catch(_){} };
    es.onerror = ()=>{ console.warn('[docbuilder] livereload error - retrying'); es.close(); setTimeout(connect,2000); };
  }
  connect();
})();`, s.config.Daemon.HTTP.LiveReloadPort)
			if _, err := w.Write([]byte(script)); err != nil {
				slog.Error("failed to write livereload script", "error", err)
			}
		})
		slog.Info("LiveReload dedicated server registered")
	}

	// LiveReload server needs no timeouts for long-lived SSE connections
	s.liveReloadServer = &http.Server{Handler: mux, ReadTimeout: 0, WriteTimeout: 0, IdleTimeout: 300 * time.Second}
	return s.startServerWithListener("livereload", s.liveReloadServer, ln)
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

// injectLiveReloadScriptWithPort is a middleware that injects the LiveReload client script
// into HTML responses, configured to connect to the specified port.
func (s *HTTPServer) injectLiveReloadScriptWithPort(next http.Handler, port int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only inject into HTML pages (not assets, API endpoints, etc.)
		path := r.URL.Path
		isHTMLPage := path == "/" || path == "" || strings.HasSuffix(path, "/") || strings.HasSuffix(path, ".html")

		if !isHTMLPage {
			// Not an HTML page, serve normally
			next.ServeHTTP(w, r)
			return
		}

		injector := newLiveReloadInjectorWithPort(w, r, port)
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
	port          int
}

func newLiveReloadInjectorWithPort(w http.ResponseWriter, _ *http.Request, port int) *liveReloadInjector {
	return &liveReloadInjector{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		maxSize:        512 * 1024, // 512KB max - typical HTML page
		port:           port,
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

// finalize must be called after the handler completes to inject the script.
func (l *liveReloadInjector) finalize() {
	if l.passthrough || len(l.buffer) == 0 {
		if !l.headerWritten {
			l.ResponseWriter.WriteHeader(l.statusCode)
		}
		return
	}

	// Inject script before </body>
	html := string(l.buffer)
	script := fmt.Sprintf(`<script src="http://localhost:%d/livereload.js"></script></body>`, l.port)
	modified := strings.Replace(html, "</body>", script, 1)

	l.ResponseWriter.Header().Del("Content-Length")
	l.ResponseWriter.WriteHeader(l.statusCode)
	_, _ = l.ResponseWriter.Write([]byte(modified))
}
