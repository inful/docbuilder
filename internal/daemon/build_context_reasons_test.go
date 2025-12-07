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
	bc := &buildContext{job: job}
	rep := &hugo2.BuildReport{}
	bc.deltaPlan = &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{"u1", "u2"}}
	if err := bc.stagePostPersist(rep, nil); err != nil {
		t.Fatalf("stagePostPersist: %v", err)
	}
	if len(rep.DeltaRepoReasons) != 2 {
		t.Fatalf("expected 2 repo reasons, got %d", len(rep.DeltaRepoReasons))
	}
	if rep.DeltaRepoReasons["u1"] != "unknown" {
		t.Fatalf("u1 reason mismatch")
	}
}
