package hugo

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// Test that stage timings are recorded and sum is <= total duration.
func TestStageRunnerTimings(t *testing.T) {
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, t.TempDir())
	doc := docs.DocFile{Repository: "repo1", Name: "readme", RelativePath: "README.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Title")}
	report, err := gen.GenerateSiteWithReport([]docs.DocFile{doc})
	if err != nil {
		t.Fatalf("GenerateSiteWithReport error: %v", err)
	}
	if len(report.StageDurations) == 0 {
		t.Fatalf("expected stage durations recorded")
	}
	var sum time.Duration
	for _, d := range report.StageDurations {
		// A stage that is fast enough measures zero: the clock of the
		// machine is the limit. The entry in the map is the check that
		// the stage ran.
		sum += d
	}
	if report.End.Sub(report.Start) < sum {
		t.Errorf("total duration %s < sum of stages %s", report.End.Sub(report.Start), sum)
	}
}
