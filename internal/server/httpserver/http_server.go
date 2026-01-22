package httpserver

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

const defaultSiteDir = "./site"

// Server manages HTTP endpoints (docs, webhooks, admin).
type Server struct {
	docsServer       *http.Server
	webhookServer    *http.Server
	adminServer      *http.Server
	liveReloadServer *http.Server
	cfg              *config.Config
	opts             Options
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

// New constructs a new HTTP server wiring instance.
func New(cfg *config.Config, runtime Runtime, opts Options) *Server {
	if opts.ForgeClients == nil {
		opts.ForgeClients = map[string]forge.Client{}
	}
	if opts.WebhookConfigs == nil {
		opts.WebhookConfigs = map[string]*config.WebhookConfig{}
	}

	s := &Server{
		cfg:                 cfg,
		opts:                opts,
		errorAdapter:        derrors.NewHTTPErrorAdapter(slog.Default()),
		vscodeFindCLI:       findCodeCLI,
		vscodeFindIPCSocket: findVSCodeIPCSocket,
	}

	adapter := &runtimeAdapter{runtime: runtime}

	// Initialize handler modules
	s.monitoringHandlers = handlers.NewMonitoringHandlers(adapter)
	s.apiHandlers = handlers.NewAPIHandlers(cfg, adapter)
	s.buildHandlers = handlers.NewBuildHandlers(adapter)
	s.webhookHandlers = handlers.NewWebhookHandlers(adapter, opts.ForgeClients, opts.WebhookConfigs)

	// Initialize middleware chain
	s.mchain = smw.Chain(slog.Default(), s.errorAdapter)

	return s
}

type runtimeAdapter struct {
	runtime Runtime
}

func (a *runtimeAdapter) GetStatus() string             { return a.runtime.GetStatus() }
func (a *runtimeAdapter) GetActiveJobs() int            { return a.runtime.GetActiveJobs() }
func (a *runtimeAdapter) GetStartTime() time.Time       { return a.runtime.GetStartTime() }
func (a *runtimeAdapter) HTTPRequestsTotal() int        { return a.runtime.HTTPRequestsTotal() }
func (a *runtimeAdapter) RepositoriesTotal() int        { return a.runtime.RepositoriesTotal() }
func (a *runtimeAdapter) LastDiscoveryDurationSec() int { return a.runtime.LastDiscoveryDurationSec() }
func (a *runtimeAdapter) LastBuildDurationSec() int     { return a.runtime.LastBuildDurationSec() }
func (a *runtimeAdapter) TriggerDiscovery() string      { return a.runtime.TriggerDiscovery() }
func (a *runtimeAdapter) TriggerBuild() string          { return a.runtime.TriggerBuild() }
func (a *runtimeAdapter) TriggerWebhookBuild(r, b string) string {
	return a.runtime.TriggerWebhookBuild(r, b)
}
func (a *runtimeAdapter) GetQueueLength() int { return a.runtime.GetQueueLength() }

// Start initializes and starts all HTTP servers.
func (s *Server) Start(ctx context.Context) error {
	if s.cfg.Daemon == nil {
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
		{name: "docs", port: s.cfg.Daemon.HTTP.DocsPort},
		{name: "webhook", port: s.cfg.Daemon.HTTP.WebhookPort},
		{name: "admin", port: s.cfg.Daemon.HTTP.AdminPort},
	}
	// Add LiveReload port if LiveReload is enabled
	if s.cfg.Build.LiveReload && s.opts.LiveReloadHub != nil {
		binds = append(binds, preBind{name: "livereload", port: s.cfg.Daemon.HTTP.LiveReloadPort})
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
	if s.cfg.Build.LiveReload && s.opts.LiveReloadHub != nil && len(binds) > 3 {
		if err := s.startLiveReloadServerWithListener(ctx, binds[3].ln); err != nil {
			return fmt.Errorf("failed to start livereload server: %w", err)
		}
		slog.Info("HTTP servers started",
			slog.Int("docs_port", s.cfg.Daemon.HTTP.DocsPort),
			slog.Int("webhook_port", s.cfg.Daemon.HTTP.WebhookPort),
			slog.Int("admin_port", s.cfg.Daemon.HTTP.AdminPort),
			slog.Int("livereload_port", s.cfg.Daemon.HTTP.LiveReloadPort))
	} else {
		slog.Info("HTTP servers started",
			slog.Int("docs_port", s.cfg.Daemon.HTTP.DocsPort),
			slog.Int("webhook_port", s.cfg.Daemon.HTTP.WebhookPort),
			slog.Int("admin_port", s.cfg.Daemon.HTTP.AdminPort))
	}
	return nil
}

// Stop gracefully shuts down all HTTP servers.
func (s *Server) Stop(ctx context.Context) error {
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
func (s *Server) startServerWithListener(kind string, srv *http.Server, ln net.Listener) error {
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
