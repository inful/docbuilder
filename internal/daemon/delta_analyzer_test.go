package daemon

import (
	"testing"
	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// fakeDeltaState reuses only needed fields (separate from skip tests for clarity)
 type fakeDeltaState struct {
	global string
	perRepo map[string]string
	commits map[string]string
 }

 func (f *fakeDeltaState) GetLastGlobalDocFilesHash() string { return f.global }
 func (f *fakeDeltaState) GetRepoDocFilesHash(u string) string { return f.perRepo[u] }
 func (f *fakeDeltaState) GetRepoLastCommit(u string) string { return f.commits[u] }

func TestDeltaAnalyzer_NoChangeFull(t *testing.T) {
	st := &fakeDeltaState{global: "g", perRepo: map[string]string{"u": "h"}, commits: map[string]string{"u": "c"}}
	repo := cfg.Repository{Name: "r", URL: "u"}
	plan := NewDeltaAnalyzer(st).Analyze("hash", []cfg.Repository{repo})
	if plan.Decision != DeltaDecisionFull || plan.Reason != "no_detected_repo_change" {
		t.Fatalf("expected full (no change) got %+v", plan)
	}
}

func TestDeltaAnalyzer_SubsetPartial(t *testing.T) {
	st := &fakeDeltaState{global: "g", perRepo: map[string]string{"u1": "h1", "u2": ""}, commits: map[string]string{"u1": "c1", "u2": "c2"}}
	repos := []cfg.Repository{{Name: "r1", URL: "u1"}, {Name: "r2", URL: "u2"}}
	plan := NewDeltaAnalyzer(st).Analyze("hash", repos)
	if plan.Decision != DeltaDecisionPartial || len(plan.ChangedRepos) != 1 || plan.ChangedRepos[0] != "u2" {
		t.Fatalf("expected partial with u2 changed, got %+v", plan)
	}
}

func TestDeltaAnalyzer_AllChangedFull(t *testing.T) {
	st := &fakeDeltaState{global: "g", perRepo: map[string]string{"u1": "", "u2": ""}, commits: map[string]string{"u1": "c1", "u2": ""}}
	repos := []cfg.Repository{{Name: "r1", URL: "u1"}, {Name: "r2", URL: "u2"}}
	plan := NewDeltaAnalyzer(st).Analyze("hash", repos)
	if plan.Decision != DeltaDecisionFull || (plan.Reason != "all_repos_changed" && plan.Reason != "all_repos_unknown_state") {
		t.Fatalf("expected full (all changed) got %+v", plan)
	}
}
