package hugo

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// Test that stage timings are recorded and sum is <= total duration.
func TestStageRunnerTimings(t *testing.T) {
	cfg := &config.Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	gen := NewGenerator(cfg, t.TempDir())
	doc := docs.DocFile{Repository: "repo1", Name: "readme", RelativePath: "README.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Title")}
	report, err := gen.GenerateSiteWithReport([]docs.DocFile{doc})
	if err != nil {
		t.Fatalf("GenerateSiteWithReport error: %v", err)
	}
	if len(report.StageDurations) == 0 {
		t.Fatalf("expected stage durations recorded")
	}
	var sum time.Duration
	for name, d := range report.StageDurations {
		if d <= 0 {
			t.Errorf("stage %s duration zero", name)
		}
		sum += d
	}
	if report.End.Sub(report.Start) < sum {
		t.Errorf("total duration %s < sum of stages %s", report.End.Sub(report.Start), sum)
	}
}
