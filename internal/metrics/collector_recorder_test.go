package metrics

import (
	"testing"
	"time"
)

// The collector of the metrics.
type fakeCollector struct {
	counters   map[string]int64
	gauges     map[string]int64
	histograms map[string][]float64
	custom     map[string]any
}

func newFakeCollector() *fakeCollector {
	return &fakeCollector{
		counters:   map[string]int64{},
		gauges:     map[string]int64{},
		histograms: map[string][]float64{},
		custom:     map[string]any{},
	}
}

func (c *fakeCollector) IncrementCounter(name string) { c.counters[name]++ }
func (c *fakeCollector) AddCounter(name string, delta int64) {
	c.counters[name] += delta
}
func (c *fakeCollector) SetGauge(name string, value int64) { c.gauges[name] = value }
func (c *fakeCollector) RecordHistogram(name string, value float64) {
	c.histograms[name] = append(c.histograms[name], value)
}
func (c *fakeCollector) SetCustomMetric(name string, value any) { c.custom[name] = value }

// The recorder of the build writes into the collector.
//
// The recorder was never given to anybody: every call went to the
// no-op recorder, and the metrics of the build stayed at zero.
func TestTheRecorderWritesIntoTheCollector(t *testing.T) {
	collector := newFakeCollector()
	recorder := CollectorRecorder{Collector: collector}

	recorder.ObserveBuildDuration(2 * time.Second)
	recorder.ObserveStageDuration("clone_repos", time.Second)
	recorder.IncBuildOutcome(BuildOutcomeSuccess)
	recorder.IncStageResult("clone_repos", ResultSuccess)
	recorder.IncCloneRepoResult(true)
	recorder.IncCloneRepoResult(false)
	recorder.SetCloneConcurrency(4)
	recorder.IncBuildRetry("clone_repos")
	recorder.IncBuildRetryExhausted("clone_repos")
	recorder.IncIssue("clone_failure", "clone_repos", "error", false)
	recorder.SetEffectiveRenderMode("always")
	recorder.IncContentTransformFailure("links")
	recorder.ObserveContentTransformDuration("links", time.Second, true)

	if collector.counters["build_outcome_success_total"] != 1 {
		t.Errorf("the outcome of the build was not counted: %+v", collector.counters)
	}
	if collector.counters["stage_result_clone_repos_success_total"] != 1 {
		t.Errorf("the result of the stage was not counted: %+v", collector.counters)
	}
	if collector.counters["clone_repo_success_total"] != 1 || collector.counters["clone_repo_failure_total"] != 1 {
		t.Errorf("the results of the clone were not counted: %+v", collector.counters)
	}
	if collector.gauges["clone_concurrency"] != 4 {
		t.Errorf("the concurrency of the clone was not recorded: %+v", collector.gauges)
	}
	if len(collector.histograms["build_duration_seconds"]) != 1 {
		t.Errorf("the duration of the build was not recorded: %+v", collector.histograms)
	}
	if collector.custom["effective_render_mode"] != "always" {
		t.Errorf("the mode of the build was not recorded: %+v", collector.custom)
	}
}
