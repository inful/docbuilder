package httpserver

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func (s *Server) startAdminServerWithListener(_ context.Context, ln net.Listener) error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc(s.cfg.Monitoring.Health.Path, s.monitoringHandlers.HandleHealthCheck)
	mux.HandleFunc("/healthz", s.monitoringHandlers.HandleHealthCheck) // Kubernetes-style alias
	// Readiness endpoint: only ready when a rendered site exists under <output>/public
	mux.HandleFunc("/ready", s.handleReadiness)
	mux.HandleFunc("/readyz", s.handleReadiness) // Kubernetes-style alias
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

	s.adminServer = &http.Server{Handler: s.mchain(mux), ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	return s.startServerWithListener("admin", s.adminServer, ln)
}

func (s *Server) handleReadiness(w http.ResponseWriter, _ *http.Request) {
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
