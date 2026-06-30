package httpserver

import (
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// Status is the minimal Runtime surface shared by every server mode.
// Docs site needs it on every page; preview and full-mode both supply it.
type Status interface {
	GetStatus() string
	GetStartTime() time.Time
	GetActiveJobs() int
}

// Triggers is an optional Runtime surface for trigger endpoints.
// nil disables discovery/build/webhook routes (preview mode).
type Triggers interface {
	TriggerDiscovery() string
	TriggerBuild() string
	TriggerWebhookBuild(forgeName, repoFullName, branch string, changedFiles []string) string
	GetQueueLength() int
}

// MetricsSource is an optional Runtime surface for the metrics endpoint.
// nil disables Prometheus/detailed metrics (preview mode).
type MetricsSource interface {
	HTTPRequestsTotal() int
	RepositoriesTotal() int
	LastDiscoveryDurationSec() int
	LastBuildDurationSec() int
}

// BuildStatus is used to render preview-mode error pages when no good build exists yet.
type BuildStatus interface {
	GetStatus() (hasError bool, err error, hasGoodBuild bool)
}

// Options configures additional server wiring that is runtime-specific.
// Triggers and Metrics are optional; pass nil when the surface is not
// available in the current runtime mode.
type Options struct {
	ForgeClients   map[string]forge.Client
	WebhookConfigs map[string]*config.WebhookConfig

	// Optional: live reload support (preview mode). Callers pass any
	// type satisfying queue.LiveReloadHub (http.Handler + Broadcast +
	// Shutdown); see internal/build/queue/build_job_metadata.go.
	LiveReloadHub queue.LiveReloadHub

	// Optional: build status tracker (preview mode).
	BuildStatus BuildStatus

	// Optional: extra admin endpoints.
	PrometheusHandler     http.Handler
	DetailedMetricsHandle http.HandlerFunc
	EnhancedHealthHandle  http.HandlerFunc
	StatusHandle          http.HandlerFunc

	// Optional: trigger surface (full daemon mode supplies a *Daemon,
	// preview passes nil so /api/build/trigger etc. are no-ops).
	Triggers Triggers

	// Optional: metrics surface (full daemon mode supplies *Daemon,
	// preview passes nil so /metrics returns zeros).
	Metrics MetricsSource
}
