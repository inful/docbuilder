package httpserver

import (
	"testing"
	"time"
)

// stubStatus / stubMetrics / stubTriggers are test implementations of the
// split Runtime surfaces. They are intentionally tiny so the test for the
// handler adapters below stays focused on the wiring, not the providers.
type stubStatus struct {
	status string
	start  time.Time
	active int
}

func (s stubStatus) GetStatus() string       { return s.status }
func (s stubStatus) GetStartTime() time.Time { return s.start }
func (s stubStatus) GetActiveJobs() int      { return s.active }

type stubMetrics struct {
	req, repos, disc, buildSec int
}

func (m stubMetrics) HTTPRequestsTotal() int        { return m.req }
func (m stubMetrics) RepositoriesTotal() int        { return m.repos }
func (m stubMetrics) LastDiscoveryDurationSec() int { return m.disc }
func (m stubMetrics) LastBuildDurationSec() int     { return m.buildSec }

type stubTriggers struct {
	queueLen       int
	triggerDisc    string
	triggerBuild   string
	triggerWebhook string
}

func (t stubTriggers) TriggerDiscovery() string { return t.triggerDisc }
func (t stubTriggers) TriggerBuild() string     { return t.triggerBuild }
func (t stubTriggers) TriggerWebhookBuild(_, _, _ string, _ []string) string {
	return t.triggerWebhook
}
func (t stubTriggers) GetQueueLength() int { return t.queueLen }

// TestMonitoringAdapter_ForwardsMetrics ensures the monitoring adapter
// proxies a non-nil MetricsSource.
func TestMonitoringAdapter_ForwardsMetrics(t *testing.T) {
	st := stubStatus{status: "running", start: time.Unix(1, 0), active: 7}
	met := stubMetrics{req: 11, repos: 3, disc: 4, buildSec: 5}
	a := &monitoringAdapter{status: st, metrics: met}

	if got := a.GetStatus(); got != "running" {
		t.Errorf("GetStatus=%q want running", got)
	}
	if got := a.GetStartTime(); !got.Equal(time.Unix(1, 0)) {
		t.Errorf("GetStartTime=%v want %v", got, time.Unix(1, 0))
	}
	if got := a.GetActiveJobs(); got != 7 {
		t.Errorf("GetActiveJobs=%d want 7", got)
	}
	if got := a.HTTPRequestsTotal(); got != 11 {
		t.Errorf("HTTPRequestsTotal=%d want 11", got)
	}
	if got := a.RepositoriesTotal(); got != 3 {
		t.Errorf("RepositoriesTotal=%d want 3", got)
	}
	if got := a.LastDiscoveryDurationSec(); got != 4 {
		t.Errorf("LastDiscoveryDurationSec=%d want 4", got)
	}
	if got := a.LastBuildDurationSec(); got != 5 {
		t.Errorf("LastBuildDurationSec=%d want 5", got)
	}
}

// TestMonitoringAdapter_PreviewWithoutMetrics ensures the adapter returns
// zeros for metrics when no MetricsSource is supplied (preview-mode wiring).
func TestMonitoringAdapter_PreviewWithoutMetrics(t *testing.T) {
	st := stubStatus{status: "preview", start: time.Time{}, active: 0}
	a := &monitoringAdapter{status: st, metrics: nil}

	if got := a.GetStatus(); got != "preview" {
		t.Errorf("GetStatus=%q want preview", got)
	}
	if got := a.HTTPRequestsTotal(); got != 0 {
		t.Errorf("HTTPRequestsTotal=%d want 0 (no metrics in preview)", got)
	}
	if got := a.LastBuildDurationSec(); got != 0 {
		t.Errorf("LastBuildDurationSec=%d want 0", got)
	}
}

// TestBuildAndWebhookAdapter_ForwardTriggers ensures both adapters share a
// single Triggers source and that build picks up GetActiveJobs from Status.
func TestBuildAndWebhookAdapter_ForwardTriggers(t *testing.T) {
	st := stubStatus{status: "running", start: time.Unix(2, 0), active: 9}
	tr := stubTriggers{queueLen: 4, triggerDisc: "d", triggerBuild: "b", triggerWebhook: "w"}
	b := &buildAdapter{status: st, triggers: tr}
	w := &webhookAdapter{triggers: tr}

	if got := b.TriggerDiscovery(); got != "d" {
		t.Errorf("TriggerDiscovery=%q want d", got)
	}
	if got := b.TriggerBuild(); got != "b" {
		t.Errorf("TriggerBuild=%q want b", got)
	}
	if got := b.GetQueueLength(); got != 4 {
		t.Errorf("GetQueueLength=%d want 4", got)
	}
	if got := b.GetActiveJobs(); got != 9 {
		t.Errorf("GetActiveJobs=%d want 9 (from Status)", got)
	}
	if got := w.TriggerWebhookBuild("forge", "repo", "main", nil); got != "w" {
		t.Errorf("TriggerWebhookBuild=%q want w", got)
	}
}

// TestBuildAndWebhookAdapter_PreviewWithoutTriggers ensures the adapters
// return zeros / "" when no Triggers source is provided.
func TestBuildAndWebhookAdapter_PreviewWithoutTriggers(t *testing.T) {
	st := stubStatus{status: "preview"}
	b := &buildAdapter{status: st, triggers: nil}
	w := &webhookAdapter{triggers: nil}

	if got := b.TriggerDiscovery(); got != "" {
		t.Errorf("TriggerDiscovery=%q want empty", got)
	}
	if got := b.GetQueueLength(); got != 0 {
		t.Errorf("GetQueueLength=%d want 0", got)
	}
	if got := b.GetActiveJobs(); got != 0 {
		t.Errorf("GetActiveJobs=%d want 0 (preview)", got)
	}
	if got := w.TriggerWebhookBuild("", "", "", nil); got != "" {
		t.Errorf("TriggerWebhookBuild=%q want empty", got)
	}
}
