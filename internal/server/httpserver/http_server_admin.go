package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func (s *Server) startAdminServerWithListener(_ context.Context, ln net.Listener) error {
	// The paths of the monitoring endpoints come from the
	// configuration. The router panics on a path without a leading
	// slash and on a path that is already served, and the panic takes
	// the whole daemon down: the paths are checked here.
	if err := s.checkMonitoringPaths(); err != nil {
		return err
	}

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc(s.cfg.Monitoring.Health.Path, s.monitoringHandlers.HandleHealthCheck)
	mux.HandleFunc("/healthz", s.monitoringHandlers.HandleHealthCheck) // Kubernetes-style alias
	// Readiness endpoint: the daemon is up and able to serve.
	mux.HandleFunc("/ready", s.handleReadiness)
	mux.HandleFunc("/readyz", s.handleReadiness) // Kubernetes-style alias
	// Render-state endpoint: ready only when a rendered site exists under <output>/public
	mux.HandleFunc("/site-ready", s.handleSiteReadiness)
	// Add enhanced health check endpoint (if daemon is available)
	if s.opts.EnhancedHealthHandle != nil {
		mux.HandleFunc("/health/detailed", s.opts.EnhancedHealthHandle)
	} else {
		mux.HandleFunc("/health/detailed", s.monitoringHandlers.HandleHealthCheck)
	}

	// Metrics endpoint
	if s.cfg.Monitoring.Metrics.Enabled {
		mux.HandleFunc(s.cfg.Monitoring.Metrics.Path, s.monitoringHandlers.HandleMetrics)
		if s.opts.DetailedMetricsHandle != nil {
			mux.HandleFunc("/metrics/detailed", s.opts.DetailedMetricsHandle)
		} else {
			mux.HandleFunc("/metrics/detailed", s.monitoringHandlers.HandleMetrics)
		}
		if s.opts.PrometheusHandler != nil {
			mux.Handle("/metrics/prometheus", s.opts.PrometheusHandler)
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
	if s.opts.StatusHandle != nil {
		mux.HandleFunc("/status", s.opts.StatusHandle)
	}

	// A page in the browser of the operator can post a form to the admin
	// port: the answer is unreadable, but the side effect happens. The
	// requests that come from another origin are refused.
	adminMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !isLoopbackOrigin(origin) {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})

	s.adminServer = &http.Server{Handler: s.mchain(adminMux), ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	return s.startServerWithListener("admin", s.adminServer, ln)
}

// adminReservedPaths are the routes of the admin server that the
// operator cannot change. A configured monitoring path that is one of
// them makes the router refuse the second registration, and the router
// panics on it.
var adminReservedPaths = []string{
	"/healthz",
	"/ready",
	"/readyz",
	"/site-ready",
	"/health/detailed",
	"/metrics/detailed",
	"/metrics/prometheus",
	"/api/daemon/status",
	"/api/daemon/config",
	"/api/discovery/trigger",
	"/api/build/trigger",
	"/api/build/status",
	"/api/repositories",
	"/status",
}

// checkMonitoringPaths refuses the monitoring paths that the router
// would panic on: a path without a leading slash, a path that is
// already served by the admin server, and the same path for the health
// and the metrics endpoint.
func (s *Server) checkMonitoringPaths() error {
	if s.cfg.Monitoring == nil {
		return nil
	}
	health := s.cfg.Monitoring.Health.Path
	metrics := s.cfg.Monitoring.Metrics.Path

	for _, p := range []struct {
		name string
		path string
	}{{"monitoring.health.path", health}, {"monitoring.metrics.path", metrics}} {
		if p.path == "" {
			continue
		}
		if !strings.HasPrefix(p.path, "/") {
			return fmt.Errorf("the %s (%q) must start with /", p.name, p.path)
		}
		if strings.ContainsAny(p.path, "{}") {
			// A path with braces is a wildcard of the router, and a
			// malformed one takes the process down when the router is
			// built.
			return fmt.Errorf("the %s (%q) may not hold a wildcard", p.name, p.path)
		}
		if slices.Contains(adminReservedPaths, p.path) {
			return fmt.Errorf("the %s (%q) is already served by the admin server", p.name, p.path)
		}
	}
	if health != "" && health == metrics {
		return fmt.Errorf("the monitoring.health.path and the monitoring.metrics.path are the same (%q)", health)
	}
	return nil
}

// isLoopbackOrigin reports whether an origin is the machine itself. The
// admin API is bound to the loopback interface by default, and a request
// from the browser of the operator comes from another origin.
func isLoopbackOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// daemonStatusRunning mirrors internal/daemon.StatusRunning. This package
// cannot import internal/daemon (the daemon imports this package), so the
// value is duplicated with a pointer to its source.
const daemonStatusRunning = "running"

// handleReadiness reports whether the daemon is up and able to serve.
//
// It deliberately does NOT depend on the rendered site. A daemon whose
// forge is unreachable, or whose first build has not finished yet, is
// still a working daemon: gating readiness on the build turns a build
// problem into a permanently unready pod (and, as a startup probe, into a
// restart loop that can never fix the build). Use /site-ready when the
// question is "is the site built".
func (s *Server) handleReadiness(w http.ResponseWriter, _ *http.Request) {
	if s.runtime == nil || s.runtime.GetStatus() != daemonStatusRunning {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready: daemon is not running"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// handleSiteReadiness reports whether a rendered site exists
// (<output.directory>/public). This is the render-state check that
// /ready used to perform; it is kept under its own path so deployments
// that gate traffic on "the site is built" can keep doing so.
func (s *Server) handleSiteReadiness(w http.ResponseWriter, _ *http.Request) {
	out := s.cfg.Output.Directory
	if out == "" {
		out = defaultSiteDir
	}
	// Combine with base_directory if set and path is relative
	if s.cfg.Output.BaseDirectory != "" && !filepath.IsAbs(out) {
		out = filepath.Join(s.cfg.Output.BaseDirectory, out)
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
