package hugo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestBuildReportStability compares a synthesized minimal successful build report against a golden
// file on disk. Only stable, non-dynamic fields are asserted: we zero / normalize timestamps and
// ignore duration numeric drift by clamping to milliseconds. The golden can be updated intentionally
// when schema additions occur (additive changes require appending keys, not removing existing ones).
func TestBuildReportStability(t *testing.T) {
	r := newBuildReport(1, 3)
	r.ClonedRepositories = 1
	r.RenderedPages = 3
	r.StageDurations["prepare_output"] = 123 * time.Millisecond
	r.recordStageResult(StagePrepareOutput, StageResultSuccess, nil)
	r.finish()
	r.deriveOutcome()
	r.ConfigHash = "deadbeef" // deterministic stub
	r.PipelineVersion = 1
	r.EffectiveRenderMode = "auto"
	ser := r.sanitizedCopy()
	// Populate optional fields to match golden defaults
	if ser.DocFilesHash == "" {
		ser.DocFilesHash = ""
	}
	if ser.DeltaDecision == "" {
		ser.DeltaDecision = ""
	}
	if ser.DeltaChangedRepos == nil {
		ser.DeltaChangedRepos = nil
	}
	if ser.DeltaRepoReasons == nil {
		ser.DeltaRepoReasons = nil
	}

	// Normalize dynamic fields
	ser.Start = time.Unix(0, 0).UTC()
	ser.End = time.Unix(0, 0).UTC()
	// Marshal
	gotJSON, err := json.MarshalIndent(ser, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "build-report.golden.json")
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var gMap, cMap map[string]any
	if err := json.Unmarshal(golden, &gMap); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	if err := json.Unmarshal(gotJSON, &cMap); err != nil {
		t.Fatalf("unmarshal current: %v", err)
	}
	// Keys we require to stay stable (subset).
	required := []string{"schema_version", "repositories", "files", "outcome", "cloned_repositories", "rendered_pages", "config_hash", "pipeline_version", "effective_render_mode"}
	for _, k := range required {
		if gMap[k] != cMap[k] {
			t.Fatalf("field %s mismatch: golden=%v current=%v", k, gMap[k], cMap[k])
		}
	}
}
