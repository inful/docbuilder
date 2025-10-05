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
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	handlers "git.home.luguber.info/inful/docbuilder/internal/server/handlers"
	derrors "git.home.luguber.info/inful/docbuilder/internal/errors"
	smw "git.home.luguber.info/inful/docbuilder/internal/server/middleware"
)

// HTTPServer manages HTTP endpoints for the daemon
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

// NewHTTPServer creates a new HTTP server instance with the specified configuration
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

func (a *daemonAdapter) GetStatus() interface{} {
	return a.daemon.GetStatus()
}

func (a *daemonAdapter) GetActiveJobs() int {
	return a.daemon.GetActiveJobs()
}

func (a *daemonAdapter) GetStartTime() time.Time {
	return a.daemon.GetStartTime()
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

// Start initializes and starts all HTTP servers
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
	return s.startDocsServerWithListener(ctx, nil)
}

// startDocsServerWithListener allows injecting a pre-bound listener (for coordinated bind checks).
func (s *HTTPServer) startDocsServerWithListener(ctx context.Context, ln net.Listener) error {
	mux := http.NewServeMux()
	// Root handler dynamically chooses between the Hugo output directory and the rendered "public" folder.
	// This lets us begin serving immediately (before a static render completes) while automatically
	// switching to the fully rendered site once available—without restarting the daemon.
	mux.Handle("/", s.mchain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := s.resolveDocsRoot()
		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})))

	// API endpoint for documentation status
	mux.HandleFunc("/api/status", s.apiHandlers.HandleDocsStatus)

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

	s.docsServer = &http.Server{Handler: mux, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	go func() {
		var err error
		if ln != nil {
			err = s.docsServer.Serve(ln)
		} else {
			err = s.docsServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
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
	return s.startWebhookServerWithListener(ctx, nil)
}

func (s *HTTPServer) startWebhookServerWithListener(ctx context.Context, ln net.Listener) error {
	mux := http.NewServeMux()

	// Webhook endpoints for each forge type
	mux.HandleFunc("/webhooks/github", s.webhookHandlers.HandleGitHubWebhook)
	mux.HandleFunc("/webhooks/gitlab", s.webhookHandlers.HandleGitLabWebhook)
	mux.HandleFunc("/webhooks/forgejo", s.webhookHandlers.HandleForgejoWebhook)

	// Generic webhook endpoint (auto-detects forge type)
	mux.HandleFunc("/webhook", s.webhookHandlers.HandleGenericWebhook)

	s.webhookServer = &http.Server{Handler: s.mchain(mux), ReadTimeout: 30 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		var err error
		if ln != nil {
			err = s.webhookServer.Serve(ln)
		} else {
			err = s.webhookServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Webhook server error", "error", err)
		}
	}()
	return nil
}

// startAdminServer starts the administrative API server
func (s *HTTPServer) startAdminServer(ctx context.Context) error {
	return s.startAdminServerWithListener(ctx, nil)
}

func (s *HTTPServer) startAdminServerWithListener(ctx context.Context, ln net.Listener) error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc(s.config.Monitoring.Health.Path, s.monitoringHandlers.HandleHealthCheck)
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
	go func() {
		var err error
		if ln != nil {
			err = s.adminServer.Serve(ln)
		} else {
			err = s.adminServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Admin server error", "error", err)
		}
	}()
	return nil
}

// prometheusOptionalHandler returns the Prometheus metrics handler. Previously
// this was gated behind a build tag; it now always returns a handler.

// inline middleware removed in favor of internal/server/middleware
