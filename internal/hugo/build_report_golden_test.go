package hugo

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

// TestBuildReportGolden ensures that the serialized JSON schema for a minimal successful build
// remains stable (excluding dynamic timestamps which are compared for presence only).
func TestBuildReportGolden(t *testing.T) {
	r := newBuildReport(context.Background(), 2, 5)
	r.ClonedRepositories = 2
	r.RenderedPages = 5
	r.StageDurations["prepare_output"] = 10 * time.Millisecond
	r.StageErrorKinds[StagePrepareOutput] = "" // no error
	r.recordStageResult(StagePrepareOutput, StageResultSuccess, nil)
	r.finish()
	r.deriveOutcome()

	ser := r.sanitizedCopy()
	jb, err := json.MarshalIndent(ser, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Basic structural assertions (rather than brittle literal string match):
	var m map[string]interface{}
	if err := json.Unmarshal(jb, &m); err != nil {
		t.Fatalf("unmarshal round trip: %v", err)
	}
	// Required keys
	required := []string{"schema_version", "repositories", "files", "start", "end", "errors", "warnings", "stage_durations", "stage_error_kinds", "cloned_repositories", "failed_repositories", "skipped_repositories", "rendered_pages", "stage_counts", "outcome", "static_rendered", "retries", "retries_exhausted", "issues"}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %s in serialized report", k)
		}
	}
	if m["outcome"].(string) != "success" {
		t.Errorf("expected outcome success, got %v", m["outcome"])
	}
	// Ensure stage_counts structure shape
	if sc, ok := m["stage_counts"].(map[string]interface{}); ok {
		if _, ok2 := sc["prepare_output"]; !ok2 {
			t.Errorf("expected prepare_output entry in stage_counts")
		}
	} else {
		t.Errorf("stage_counts not an object")
	}
	_ = filepath.Base // ensure filepath imported if future path-based golden file added
}
