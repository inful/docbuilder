package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// HTTPServer manages HTTP endpoints for the daemon
type HTTPServer struct {
	docsServer    *http.Server
	webhookServer *http.Server
	adminServer   *http.Server
	config        *config.Config
	daemon        *Daemon // Reference to main daemon service
}

// NewHTTPServer creates a new HTTP server manager
func NewHTTPServer(cfg *config.Config, daemon *Daemon) *HTTPServer {
	return &HTTPServer{
		config: cfg,
		daemon: daemon,
	}
}

// Start initializes and starts all HTTP servers
func (s *HTTPServer) Start(ctx context.Context) error {
	if s.config.Daemon == nil {
		return fmt.Errorf("daemon configuration required for HTTP servers")
	}

	// Start documentation server
	if err := s.startDocsServer(ctx); err != nil {
		return fmt.Errorf("failed to start docs server: %w", err)
	}

	// Start webhook server
	if err := s.startWebhookServer(ctx); err != nil {
		return fmt.Errorf("failed to start webhook server: %w", err)
	}

	// Start admin server
	if err := s.startAdminServer(ctx); err != nil {
		return fmt.Errorf("failed to start admin server: %w", err)
	}

	slog.Info("HTTP servers started",
		slog.Int("docs_port", s.config.Daemon.HTTP.DocsPort),
		slog.Int("webhook_port", s.config.Daemon.HTTP.WebhookPort),
		slog.Int("admin_port", s.config.Daemon.HTTP.AdminPort))

	return nil
}

// Stop gracefully shuts down all HTTP servers
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

// startDocsServer starts the documentation serving server
func (s *HTTPServer) startDocsServer(ctx context.Context) error {
	mux := http.NewServeMux()
	// Root handler dynamically chooses between the Hugo output directory and the rendered "public" folder.
	// This lets us begin serving immediately (before a static render completes) while automatically
	// switching to the fully rendered site once available—without restarting the daemon.
	mux.Handle("/", s.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := s.resolveDocsRoot()
		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})))

	// API endpoint for documentation status
	mux.HandleFunc("/api/status", s.handleDocsStatus)

	// LiveReload endpoints (SSE + script) if enabled
	if s.config.Build.LiveReload && s.daemon != nil && s.daemon.liveReload != nil {
		mux.Handle("/livereload", s.daemon.liveReload)
		mux.HandleFunc("/livereload.js", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			if _, err := w.Write([]byte(LiveReloadScript)); err != nil {
				slog.Error("failed to write livereload script", "error", err)
			}
		})
		slog.Info("LiveReload HTTP endpoints registered")
	}

	s.docsServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Daemon.HTTP.DocsPort),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := s.docsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Documentation server error", "error", err)
		}
	}()

	return nil
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
	public := filepath.Join(out, "public")
	if st, err := os.Stat(public); err == nil && st.IsDir() {
		return public
	}
	return out
}

// startWebhookServer starts the webhook handling server
func (s *HTTPServer) startWebhookServer(ctx context.Context) error {
	mux := http.NewServeMux()

	// Webhook endpoints for each forge type
	mux.HandleFunc("/webhooks/github", s.handleGitHubWebhook)
	mux.HandleFunc("/webhooks/gitlab", s.handleGitLabWebhook)
	mux.HandleFunc("/webhooks/forgejo", s.handleForgejoWebhook)

	// Generic webhook endpoint (auto-detects forge type)
	mux.HandleFunc("/webhook", s.handleGenericWebhook)

	s.webhookServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Daemon.HTTP.WebhookPort),
		Handler:      s.loggingMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := s.webhookServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Webhook server error", "error", err)
		}
	}()

	return nil
}

// startAdminServer starts the administrative API server
func (s *HTTPServer) startAdminServer(ctx context.Context) error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc(s.config.Monitoring.Health.Path, s.handleHealthCheck)
	// Add enhanced health check endpoint
	mux.HandleFunc("/health/detailed", s.daemon.EnhancedHealthHandler)

	// Metrics endpoint
	if s.config.Monitoring.Metrics.Enabled {
		mux.HandleFunc(s.config.Monitoring.Metrics.Path, s.handleMetrics)
		mux.HandleFunc("/metrics/detailed", s.daemon.metrics.MetricsHandler)
		if h := prometheusOptionalHandler(); h != nil {
			mux.Handle("/metrics/prometheus", h)
		}
	}

	// Administrative endpoints
	mux.HandleFunc("/api/daemon/status", s.handleDaemonStatus)
	mux.HandleFunc("/api/daemon/config", s.handleDaemonConfig)
	mux.HandleFunc("/api/discovery/trigger", s.handleTriggerDiscovery)
	mux.HandleFunc("/api/build/trigger", s.handleTriggerBuild)
	mux.HandleFunc("/api/build/status", s.handleBuildStatus)
	mux.HandleFunc("/api/repositories", s.handleRepositories)

	// Status page endpoint (HTML and JSON)
	mux.HandleFunc("/status", s.daemon.StatusHandler)

	s.adminServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Daemon.HTTP.AdminPort),
		Handler:      s.loggingMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := s.adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Admin server error", "error", err)
		}
	}()

	return nil
}

// prometheusOptionalHandler returns the Prometheus metrics handler. Previously
// this was gated behind a build tag; it now always returns a handler.

// Middleware for request logging
func (s *HTTPServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		slog.Info("HTTP request",
			logfields.Method(r.Method),
			logfields.Path(r.URL.Path),
			logfields.Status(wrapped.statusCode),
			slog.Duration("duration", duration),
			logfields.UserAgent(r.UserAgent()),
			logfields.RemoteAddr(r.RemoteAddr))
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Documentation status endpoint
func (s *HTTPServer) handleDocsStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := map[string]interface{}{
		"status":      "ok",
		"title":       s.config.Hugo.Title,
		"description": s.config.Hugo.Description,
		"theme":       s.config.Hugo.Theme,
		"base_url":    s.config.Hugo.BaseURL,
		"output_dir":  s.config.Output.Directory,
		"timestamp":   time.Now().UTC(),
	}

	if err := writeJSONPretty(w, r, http.StatusOK, status); err != nil {
		slog.Error("Failed to write daemon status response", "error", err)
	}
}

// Health check endpoint
func (s *HTTPServer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "2.0", // TODO: Get from build info
		"uptime":    time.Since(s.daemon.startTime).Seconds(),
	}

	// Check daemon health
	if s.daemon != nil {
		health["daemon_status"] = s.daemon.GetStatus()
		health["active_jobs"] = s.daemon.GetActiveJobs()
	}

	if err := writeJSONPretty(w, r, http.StatusOK, health); err != nil {
		slog.Error("Failed to write health response", "error", err)
	}
}

// Metrics endpoint (placeholder)
func (s *HTTPServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement Prometheus-style metrics
	metrics := map[string]interface{}{
		"http_requests_total":     0, // TODO: Implement counters
		"active_jobs":             s.daemon.GetActiveJobs(),
		"last_discovery_duration": 0, // TODO: Track discovery timing
		"last_build_duration":     0, // TODO: Track build timing
		"repositories_total":      0, // TODO: Count managed repositories
	}

	if err := writeJSONPretty(w, r, http.StatusOK, metrics); err != nil {
		slog.Error("Failed to write metrics response", "error", err)
	}
}

// Daemon status endpoint
func (s *HTTPServer) handleDaemonStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := map[string]interface{}{
		"status":     s.daemon.GetStatus(),
		"uptime":     time.Since(s.daemon.startTime).Seconds(),
		"start_time": s.daemon.startTime,
		"config": map[string]interface{}{
			"forges_count":      len(s.config.Forges),
			"sync_schedule":     s.config.Daemon.Sync.Schedule,
			"concurrent_builds": s.config.Daemon.Sync.ConcurrentBuilds,
			"queue_size":        s.config.Daemon.Sync.QueueSize,
		},
	}

	if err := writeJSONPretty(w, r, http.StatusOK, status); err != nil {
		slog.Error("failed to encode daemon status", "error", err)
	}
}

// Daemon configuration endpoint
func (s *HTTPServer) handleDaemonConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return sanitized configuration (no secrets)
	sanitized := s.sanitizeConfig(s.config)

	if err := writeJSONPretty(w, r, http.StatusOK, sanitized); err != nil {
		slog.Error("failed to encode daemon config", "error", err)
	}
}

// Trigger discovery endpoint
func (s *HTTPServer) handleTriggerDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement discovery triggering
	if s.daemon != nil {
		jobID := s.daemon.TriggerDiscovery()
		response := map[string]interface{}{
			"status": "triggered",
			"job_id": jobID,
		}
		if err := writeJSON(w, http.StatusOK, response); err != nil {
			slog.Error("failed to encode discovery trigger response", "error", err)
		}
	} else {
		http.Error(w, "Daemon not available", http.StatusServiceUnavailable)
	}
}

// Trigger build endpoint
func (s *HTTPServer) handleTriggerBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement build triggering
	if s.daemon != nil {
		jobID := s.daemon.TriggerBuild()
		response := map[string]interface{}{
			"status": "triggered",
			"job_id": jobID,
		}
		if err := writeJSON(w, http.StatusOK, response); err != nil {
			slog.Error("failed to encode build trigger response", "error", err)
		}
	} else {
		http.Error(w, "Daemon not available", http.StatusServiceUnavailable)
	}
}

// Build status endpoint
func (s *HTTPServer) handleBuildStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement build status tracking
	status := map[string]interface{}{
		"queue_length": s.daemon.GetQueueLength(),
		"active_jobs":  s.daemon.GetActiveJobs(),
		"last_build":   nil, // TODO: Get last build info
	}

	if err := writeJSONPretty(w, r, http.StatusOK, status); err != nil {
		slog.Error("failed to encode build status", "error", err)
	}
}

// Repositories endpoint
func (s *HTTPServer) handleRepositories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement repository listing from state
	repos := map[string]interface{}{
		"repositories":   []interface{}{}, // TODO: Get from daemon state
		"total":          0,
		"last_discovery": nil, // TODO: Get last discovery time
	}

	if err := writeJSONPretty(w, r, http.StatusOK, repos); err != nil {
		slog.Error("failed to encode repositories", "error", err)
	}
}

// sanitizeConfig removes sensitive information from configuration
func (s *HTTPServer) sanitizeConfig(cfg *config.Config) map[string]interface{} {
	sanitized := map[string]interface{}{
		"version": cfg.Version,
		"hugo": map[string]interface{}{
			"title":       cfg.Hugo.Title,
			"description": cfg.Hugo.Description,
			"base_url":    cfg.Hugo.BaseURL,
			"theme":       cfg.Hugo.Theme,
		},
		"output": cfg.Output,
		"daemon": map[string]interface{}{
			"http":    cfg.Daemon.HTTP,
			"sync":    cfg.Daemon.Sync,
			"storage": cfg.Daemon.Storage,
		},
		"filtering":  cfg.Filtering,
		"versioning": cfg.Versioning,
		"monitoring": cfg.Monitoring,
	}

	// Add forge info without sensitive data
	var forges []map[string]interface{}
	for _, forge := range cfg.Forges {
		sanitizedForge := map[string]interface{}{
			"name":          forge.Name,
			"type":          forge.Type,
			"api_url":       forge.APIURL,
			"base_url":      forge.BaseURL,
			"organizations": forge.Organizations,
			"groups":        forge.Groups,
		}
		forges = append(forges, sanitizedForge)
	}
	sanitized["forges"] = forges

	return sanitized
}

// Webhook handlers (stubs for now)
func (s *HTTPServer) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	s.handleWebhookRequest(w, r, "github")
}

func (s *HTTPServer) handleGitLabWebhook(w http.ResponseWriter, r *http.Request) {
	s.handleWebhookRequest(w, r, "gitlab")
}

func (s *HTTPServer) handleForgejoWebhook(w http.ResponseWriter, r *http.Request) {
	s.handleWebhookRequest(w, r, "forgejo")
}

func (s *HTTPServer) handleGenericWebhook(w http.ResponseWriter, r *http.Request) {
	// Auto-detect forge type from headers
	forgeType := s.detectForgeType(r)
	s.handleWebhookRequest(w, r, forgeType)
}

func (s *HTTPServer) handleWebhookRequest(w http.ResponseWriter, r *http.Request, forgeType string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement webhook processing with daemon
	slog.Info("Webhook received",
		logfields.ForgeType(forgeType),
		logfields.ContentLength(r.ContentLength),
		logfields.UserAgent(r.UserAgent()))

	// For now, just acknowledge receipt
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil { //nolint:errcheck
		slog.Error("Failed to write OK response", "error", err)
	}
}

func (s *HTTPServer) detectForgeType(r *http.Request) string {
	// Detect based on headers and user agent
	userAgent := strings.ToLower(r.UserAgent())

	if strings.Contains(userAgent, "github") || r.Header.Get("X-GitHub-Event") != "" {
		return "github"
	}
	if strings.Contains(userAgent, "gitlab") || r.Header.Get("X-Gitlab-Event") != "" {
		return "gitlab"
	}
	if strings.Contains(userAgent, "forgejo") || strings.Contains(userAgent, "gitea") {
		return "forgejo"
	}

	return "unknown"
}
