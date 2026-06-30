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
//
// status is the always-required Status surface. opts.Triggers and
// opts.Metrics are optional; pass nil to disable trigger/metrics
// routes (preview-mode wiring).
func New(cfg *config.Config, status Status, opts Options) *Server {
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

	// Compose one small adapter per handler group. Each adapter forwards
	// to the optional surface if present, and returns zero values when
	// the surface is nil (preview-mode wiring).
	mon := &monitoringAdapter{status: status, metrics: opts.Metrics}
	api := &apiAdapter{status: status}
	bld := &buildAdapter{status: status, triggers: opts.Triggers}
	wh := &webhookAdapter{triggers: opts.Triggers}

	// Initialize handler modules
	s.monitoringHandlers = handlers.NewMonitoringHandlers(mon)
	s.apiHandlers = handlers.NewAPIHandlers(cfg, api)
	s.buildHandlers = handlers.NewBuildHandlers(bld)
	s.webhookHandlers = handlers.NewWebhookHandlers(wh, opts.ForgeClients, opts.WebhookConfigs)

	// Initialize middleware chain
	s.mchain = smw.Chain(slog.Default(), s.errorAdapter)

	return s
}

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
			bindErrs = append(bindErrs, derrors.WrapError(err, derrors.CategoryNetwork, "pre-bind failed").
				WithContext("server", binds[i].name).
				WithContext("port", binds[i].port).
				Build())
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
		return derrors.WrapError(errors.Join(bindErrs...), derrors.CategoryNetwork, "http startup failed").Build()
	}

	// All ports bound successfully – now start servers handing them their pre-bound listeners.
	if err := s.startDocsServerWithListener(ctx, binds[0].ln); err != nil {
		return derrors.WrapError(err, derrors.CategoryNetwork, "failed to start docs server").Build()
	}
	if err := s.startWebhookServerWithListener(ctx, binds[1].ln); err != nil {
		return derrors.WrapError(err, derrors.CategoryNetwork, "failed to start webhook server").Build()
	}
	if err := s.startAdminServerWithListener(ctx, binds[2].ln); err != nil {
		return derrors.WrapError(err, derrors.CategoryNetwork, "failed to start admin server").Build()
	}

	// Start LiveReload server if enabled
	if s.cfg.Build.LiveReload && s.opts.LiveReloadHub != nil && len(binds) > 3 {
		if err := s.startLiveReloadServerWithListener(ctx, binds[3].ln); err != nil {
			return derrors.WrapError(err, derrors.CategoryNetwork, "failed to start livereload server").Build()
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
			errs = append(errs, derrors.WrapError(err, derrors.CategoryNetwork, "livereload server shutdown").Build())
		}
	}

	if s.adminServer != nil {
		if err := s.adminServer.Shutdown(ctx); err != nil {
			errs = append(errs, derrors.WrapError(err, derrors.CategoryNetwork, "admin server shutdown").Build())
		}
	}

	if s.webhookServer != nil {
		if err := s.webhookServer.Shutdown(ctx); err != nil {
			errs = append(errs, derrors.WrapError(err, derrors.CategoryNetwork, "webhook server shutdown").Build())
		}
	}

	if s.docsServer != nil {
		if err := s.docsServer.Shutdown(ctx); err != nil {
			errs = append(errs, derrors.WrapError(err, derrors.CategoryNetwork, "docs server shutdown").Build())
		}
	}

	if len(errs) > 0 {
		return derrors.WrapError(errors.Join(errs...), derrors.CategoryNetwork, "shutdown errors").Build()
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
