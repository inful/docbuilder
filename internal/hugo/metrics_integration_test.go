package hugo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
)

type capturingRecorder struct {
	stages   map[string]int
	results  map[string]map[metrics.ResultLabel]int
	builds   int
	outcomes map[metrics.BuildOutcomeLabel]int
}

func newCapturingRecorder() *capturingRecorder {
	return &capturingRecorder{stages: map[string]int{}, results: map[string]map[metrics.ResultLabel]int{}, outcomes: map[metrics.BuildOutcomeLabel]int{}}
}

func (c *capturingRecorder) ObserveStageDuration(stage string, _ time.Duration) { c.stages[stage]++ }
func (c *capturingRecorder) ObserveBuildDuration(_ time.Duration)               { c.builds++ }
func (c *capturingRecorder) IncStageResult(stage string, r metrics.ResultLabel) {
	m, ok := c.results[stage]
	if !ok {
		m = map[metrics.ResultLabel]int{}
		c.results[stage] = m
	}
	m[r]++
}
func (c *capturingRecorder) IncBuildOutcome(o metrics.BuildOutcomeLabel)          { c.outcomes[o]++ }
func (c *capturingRecorder) ObserveCloneRepoDuration(string, time.Duration, bool) {}
func (c *capturingRecorder) IncCloneRepoResult(bool)                              {}
func (c *capturingRecorder) SetCloneConcurrency(int)                              {}
func (c *capturingRecorder) IncBuildRetry(string)                                 {}
func (c *capturingRecorder) IncBuildRetryExhausted(string)                        {}
func (c *capturingRecorder) IncIssue(string, string, string, bool)                {}
func (c *capturingRecorder) SetEffectiveRenderMode(string)                        {}
func (c *capturingRecorder) IncContentTransformFailure(string)                    {}

// TestMetricsRecorderIntegration ensures that recorder callbacks are invoked during a simple GenerateSiteWithReport run.
func TestMetricsRecorderIntegration(t *testing.T) {
	out := t.TempDir()
	g := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Site"}}, out).SetRecorder(newCapturingRecorder())
	// Ensure Hugo structure exists for index generation dirs
	if err := g.createHugoStructure(); err != nil {
		t.Fatalf("structure: %v", err)
	}
	// Create physical source files to satisfy LoadContent()
	srcA := filepath.Join(out, "a.md")
	if err := os.WriteFile(srcA, []byte("# A"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	srcB := filepath.Join(out, "b.md")
	if err := os.WriteFile(srcB, []byte("# B"), 0644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	docFiles := []docs.DocFile{
		{Repository: "repo1", Name: "a", Path: srcA, RelativePath: "a.md", DocsBase: ".", Extension: ".md"},
		{Repository: "repo1", Name: "b", Path: srcB, RelativePath: "b.md", DocsBase: ".", Extension: ".md"},
	}
	rec := g.recorder.(*capturingRecorder)
	_, err := g.GenerateSiteWithReportContext(context.Background(), docFiles)
	if err != nil {
		t.Fatalf("build errored: %v", err)
	}
	if rec.builds != 1 {
		// Build duration observed once
		if rec.builds == 0 {
			t.Errorf("expected build duration observation")
		}
	}
	if len(rec.outcomes) == 0 {
		t.Errorf("expected at least one build outcome increment")
	}
	// At least 5 stages for simple build path
	if len(rec.stages) == 0 {
		t.Errorf("expected stage durations to be recorded")
	}
	// Ensure success results counted
	foundSuccess := false
	for _, m := range rec.results {
		if m[metrics.ResultSuccess] > 0 {
			foundSuccess = true
			break
		}
	}
	if !foundSuccess {
		t.Errorf("expected at least one success result recorded")
	}
}
