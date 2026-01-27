package httpserver

import (
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// Runtime is the minimal interface required by shared HTTP handlers.
// It intentionally matches the interfaces in internal/server/handlers.
type Runtime interface {
	GetStatus() string
	GetActiveJobs() int
	GetStartTime() time.Time

	HTTPRequestsTotal() int
	RepositoriesTotal() int
	LastDiscoveryDurationSec() int
	LastBuildDurationSec() int

	TriggerDiscovery() string
	TriggerBuild() string
	TriggerWebhookBuild(forgeName, repoFullName, branch string, changedFiles []string) string
	GetQueueLength() int
}

// BuildStatus is used to render preview-mode error pages when no good build exists yet.
type BuildStatus interface {
	GetStatus() (hasError bool, err error, hasGoodBuild bool)
}

// LiveReloadHub supports the LiveReload SSE endpoint and broadcast notifications.
type LiveReloadHub interface {
	http.Handler
	Broadcast(hash string)
	Shutdown()
}

// Options configures additional server wiring that is runtime-specific.
type Options struct {
	ForgeClients   map[string]forge.Client
	WebhookConfigs map[string]*config.WebhookConfig

	// Optional: live reload support (preview mode).
	LiveReloadHub LiveReloadHub

	// Optional: build status tracker (preview mode).
	BuildStatus BuildStatus

	// Optional: extra admin endpoints.
	PrometheusHandler     http.Handler
	DetailedMetricsHandle http.HandlerFunc
	EnhancedHealthHandle  http.HandlerFunc
	StatusHandle          http.HandlerFunc
}
