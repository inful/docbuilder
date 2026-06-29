package httpserver

import "time"

// monitoringAdapter proxies Status + optional MetricsSource to the narrow
// handlers.DaemonInterface (8 methods). When metrics is nil (preview
// mode), the *Metrics family returns zero values.
type monitoringAdapter struct {
	status  Status
	metrics MetricsSource
}

func (a *monitoringAdapter) GetStatus() string       { return a.status.GetStatus() }
func (a *monitoringAdapter) GetStartTime() time.Time { return a.status.GetStartTime() }
func (a *monitoringAdapter) GetActiveJobs() int      { return a.status.GetActiveJobs() }
func (a *monitoringAdapter) HTTPRequestsTotal() int {
	if a.metrics == nil {
		return 0
	}
	return a.metrics.HTTPRequestsTotal()
}

func (a *monitoringAdapter) RepositoriesTotal() int {
	if a.metrics == nil {
		return 0
	}
	return a.metrics.RepositoriesTotal()
}

func (a *monitoringAdapter) LastDiscoveryDurationSec() int {
	if a.metrics == nil {
		return 0
	}
	return a.metrics.LastDiscoveryDurationSec()
}

func (a *monitoringAdapter) LastBuildDurationSec() int {
	if a.metrics == nil {
		return 0
	}
	return a.metrics.LastBuildDurationSec()
}

// apiAdapter proxies Status to the narrow handlers.DaemonAPIInterface.
type apiAdapter struct {
	status Status
}

func (a *apiAdapter) GetStatus() string       { return a.status.GetStatus() }
func (a *apiAdapter) GetStartTime() time.Time { return a.status.GetStartTime() }

// buildAdapter proxies Status + optional Triggers to the narrow
// handlers.DaemonBuildInterface (4 methods). When triggers is nil
// (preview mode), the trigger methods return "" / 0.
type buildAdapter struct {
	status   Status
	triggers Triggers
}

func (a *buildAdapter) TriggerDiscovery() string {
	if a.triggers == nil {
		return ""
	}
	return a.triggers.TriggerDiscovery()
}

func (a *buildAdapter) TriggerBuild() string {
	if a.triggers == nil {
		return ""
	}
	return a.triggers.TriggerBuild()
}

func (a *buildAdapter) TriggerWebhookBuild(forgeName, repoFullName, branch string, changedFiles []string) string {
	if a.triggers == nil {
		return ""
	}
	return a.triggers.TriggerWebhookBuild(forgeName, repoFullName, branch, changedFiles)
}

func (a *buildAdapter) GetQueueLength() int {
	if a.triggers == nil {
		return 0
	}
	return a.triggers.GetQueueLength()
}

func (a *buildAdapter) GetActiveJobs() int { return a.status.GetActiveJobs() }

// webhookAdapter proxies the optional Triggers surface to the narrow
// handlers.WebhookTrigger. nil-safe.
type webhookAdapter struct {
	triggers Triggers
}

func (a *webhookAdapter) TriggerWebhookBuild(forgeName, repoFullName, branch string, changedFiles []string) string {
	if a.triggers == nil {
		return ""
	}
	return a.triggers.TriggerWebhookBuild(forgeName, repoFullName, branch, changedFiles)
}
