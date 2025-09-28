package daemon

import (
    "testing"
    hugo2 "git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// dummy generator to bypass real site generation
type dummyGenerator struct { *hugo2.Generator }

func TestBuildContextDeltaRepoReasonsPropagation(t *testing.T) {
    job := &BuildJob{Metadata: map[string]interface{}{}}
    bc := &buildContext{job: job}
    // Simulate delta plan with reasons placed in metadata by stageDeltaAnalysis
    job.Metadata["delta_repo_reasons"] = map[string]string{"u1": "unknown", "u2": "quick_hash_diff"}
    rep := &hugo2.BuildReport{}
    bc.deltaPlan = &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{"u1","u2"}}
    if err := bc.stagePostPersist(rep, nil); err != nil { t.Fatalf("stagePostPersist: %v", err) }
    if len(rep.DeltaRepoReasons) != 2 { t.Fatalf("expected 2 repo reasons, got %d", len(rep.DeltaRepoReasons)) }
    if rep.DeltaRepoReasons["u1"] != "unknown" { t.Fatalf("u1 reason mismatch") }
}
