package daemon

import (
	"testing"

	hugo2 "git.home.luguber.info/inful/docbuilder/internal/hugo"
)

func TestBuildContextDeltaRepoReasonsPropagation(t *testing.T) {
	job := &BuildJob{
		TypedMeta: &BuildJobMetadata{
			DeltaRepoReasons: map[string]string{"u1": "unknown", "u2": "quick_hash_diff"},
		},
	}

	// Test DeltaManager.AttachDeltaMetadata directly
	report := &hugo2.BuildReport{}
	deltaPlan := &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{"u1", "u2"}}

	dm := NewDeltaManager()
	dm.AttachDeltaMetadata(report, deltaPlan, job)

	if len(report.DeltaRepoReasons) != 2 {
		t.Fatalf("expected 2 repo reasons, got %d", len(report.DeltaRepoReasons))
	}
	if report.DeltaRepoReasons["u1"] != "unknown" {
		t.Fatalf("u1 reason mismatch: got %s", report.DeltaRepoReasons["u1"])
	}
	if report.DeltaRepoReasons["u2"] != "quick_hash_diff" {
		t.Fatalf("u2 reason mismatch: got %s", report.DeltaRepoReasons["u2"])
	}
}
