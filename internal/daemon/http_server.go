package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	handlers "git.home.luguber.info/inful/docbuilder/internal/server/handlers"
	smw "git.home.luguber.info/inful/docbuilder/internal/server/middleware"
)

// HTTPServer manages HTTP endpoints (docs, webhooks, admin) for the daemon.
type HTTPServer struct {
	docsServer       *http.Server
	webhookServer    *http.Server
	adminServer      *http.Server
	liveReloadServer *http.Server
	config           *config.Config
	daemon           *Daemon // Reference to main daemon service
	errorAdapter     *derrors.HTTPErrorAdapter

	// VS Code edit link behavior dependencies (injected for tests).
	vscodeFindCLI       func(context.Context) string
	vscodeFindIPCSocket func() string
	vscodeRunCLI        func(ctx context.Context, codeCmd string, args []string, env []string) (stdout string, stderr string, err error)
	// If nil, defaults are used. If empty slice, retries are disabled.
	vscodeOpenBackoffs []time.Duration

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
		config:              cfg,
		daemon:              daemon,
		errorAdapter:        derrors.NewHTTPErrorAdapter(slog.Default()),
		vscodeFindCLI:       findCodeCLI,
		vscodeFindIPCSocket: findVSCodeIPCSocket,
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

// HTTPRequestsTotal is a metrics bridge for MonitoringHandlers.
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
	lc := net.ListenConfig{}
	for i := range binds {
		addr := fmt.Sprintf(":%d", binds[i].port)
		ln, err := lc.Listen(ctx, "tcp", addr)
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
