package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// --- Quick hash specific tests ---

// computeQuickHash replicates analyzer quick hash logic for test setup.
func computeQuickHash(t *testing.T, repoRoot string) string {
	t.Helper()
	docRoots := []string{"docs", "documentation"}
	paths := []string{}
	for _, dr := range docRoots {
		base := filepath.Join(repoRoot, dr)
		fi, err := os.Stat(base)
		if err != nil || !fi.IsDir() { continue }
		filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() { return nil }
			ln := strings.ToLower(d.Name())
			if strings.HasSuffix(ln, ".md") || strings.HasSuffix(ln, ".markdown") {
				rel, rerr := filepath.Rel(repoRoot, p)
				if rerr == nil { paths = append(paths, filepath.ToSlash(rel)) }
			}
			return nil
		})
	}
	if len(paths) == 0 { return "" }
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths { h.Write([]byte(p)); h.Write([]byte{0}) }
	return hex.EncodeToString(h.Sum(nil))
}

// Test single repo unchanged: quick hash matches stored -> no change (full rebuild with no_detected_repo_change)
func TestDeltaAnalyzer_QuickHashSingleRepoUnchanged(t *testing.T) {
	tmp := t.TempDir()
	repoName := "repo1"
	repoURL := "https://example.com/org/repo1.git"
	repoRoot := filepath.Join(tmp, repoName)
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil { t.Fatalf("mkdir: %v", err) }
	if err := os.WriteFile(filepath.Join(repoRoot, "docs", "intro.md"), []byte("# Intro"), 0o644); err != nil { t.Fatalf("write: %v", err) }
	stored := computeQuickHash(t, repoRoot)
	st := &fakeDeltaState{perRepo: map[string]string{repoURL: stored}, commits: map[string]string{repoURL: "deadbeef"}}
	repo := cfg.Repository{Name: repoName, URL: repoURL}
	plan := NewDeltaAnalyzer(st).WithWorkspace(tmp).Analyze("hash", []cfg.Repository{repo})
	if plan.Decision != DeltaDecisionFull || plan.Reason != "no_detected_repo_change" {
		t.Fatalf("expected full no change, got %+v", plan)
	}
}

// Test subset changed: one repo unchanged, second repo modified after stored hash snapshot -> partial
func TestDeltaAnalyzer_QuickHashSubsetChanged(t *testing.T) {
	tmp := t.TempDir()
	// repo A (unchanged)
	repoAName := "repoA"
	repoAURL := "https://example.com/org/repoA.git"
	repoARoot := filepath.Join(tmp, repoAName)
	os.MkdirAll(filepath.Join(repoARoot, "docs"), 0o755)
	os.WriteFile(filepath.Join(repoARoot, "docs", "a1.md"), []byte("# A1"), 0o644)
	hashA := computeQuickHash(t, repoARoot)
	// repo B (will change after snapshot)
	repoBName := "repoB"
	repoBURL := "https://example.com/org/repoB.git"
	repoBRoot := filepath.Join(tmp, repoBName)
	os.MkdirAll(filepath.Join(repoBRoot, "docs"), 0o755)
	os.WriteFile(filepath.Join(repoBRoot, "docs", "b1.md"), []byte("# B1"), 0o644)
	hashB := computeQuickHash(t, repoBRoot)
	// mutate repo B
	os.WriteFile(filepath.Join(repoBRoot, "docs", "b2.md"), []byte("# B2"), 0o644)
	st := &fakeDeltaState{perRepo: map[string]string{repoAURL: hashA, repoBURL: hashB}, commits: map[string]string{repoAURL: "c1", repoBURL: "c2"}}
	repos := []cfg.Repository{{Name: repoAName, URL: repoAURL}, {Name: repoBName, URL: repoBURL}}
	plan := NewDeltaAnalyzer(st).WithWorkspace(tmp).Analyze("hash", repos)
	if plan.Decision != DeltaDecisionPartial || len(plan.ChangedRepos) != 1 || plan.ChangedRepos[0] != repoBURL {
		t.Fatalf("expected partial with repoB changed, got %+v", plan)
	}
}
