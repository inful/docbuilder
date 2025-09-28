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

func TestDeltaAnalyzer_NoOpFullFallback(t *testing.T) {
	st := &fakeDeltaState{global: "g", perRepo: map[string]string{"u": "h"}, commits: map[string]string{"u": "c"}}
	repo := cfg.Repository{Name: "r", URL: "u"}
	plan := NewDeltaAnalyzer(st).Analyze("hash", []cfg.Repository{repo})
	if plan.Decision != DeltaDecisionFull || plan.Reason == "" {
		// Reason should explain fallback
		t.Fatalf("expected full fallback with reason, got %+v", plan)
	}
}
