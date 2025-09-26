package metrics

import (
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
)

func TestPrometheusRecorder(t *testing.T) {
	reg := prom.NewRegistry()
	pr := NewPrometheusRecorder(reg)
	pr.ObserveStageDuration("copy_content", 150*time.Millisecond)
	pr.ObserveBuildDuration(500 * time.Millisecond)
	pr.IncStageResult("copy_content", ResultSuccess)
	pr.IncBuildOutcome("success")
	// Basic scrape to ensure metrics encode without panic
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if len(mfs) == 0 {
		t.Fatalf("expected metrics, got none")
	}
}
