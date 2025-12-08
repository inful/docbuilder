package delta

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
	global  string
	perRepo map[string]string
	commits map[string]string
}

func (f *fakeDeltaState) GetLastGlobalDocFilesHash() string   { return f.global }
func (f *fakeDeltaState) GetRepoDocFilesHash(u string) string { return f.perRepo[u] }
func (f *fakeDeltaState) GetRepoLastCommit(u string) string   { return f.commits[u] }

func TestDeltaAnalyzer_NoChangeFull(t *testing.T) {
	st := &fakeDeltaState{global: "g", perRepo: map[string]string{"u": "h"}, commits: map[string]string{"u": "c"}}
	repo := cfg.Repository{Name: "r", URL: "u"}
	plan := NewDeltaAnalyzer(st).Analyze("hash", []cfg.Repository{repo})
	if plan.Decision != DeltaDecisionFull || plan.Reason != "no_detected_repo_change" {
		t.Fatalf("expected full (no change) got %+v", plan)
	}
	if plan.RepoReasons != nil {
		if plan.RepoReasons["u"] != RepoReasonUnchanged {
			t.Fatalf("expected repo reason '%s' got %s", RepoReasonUnchanged, plan.RepoReasons["u"])
		}
	}
}

func TestDeltaAnalyzer_SubsetPartial(t *testing.T) {
	st := &fakeDeltaState{global: "g", perRepo: map[string]string{"u1": "h1", "u2": ""}, commits: map[string]string{"u1": "c1", "u2": "c2"}}
	repos := []cfg.Repository{{Name: "r1", URL: "u1"}, {Name: "r2", URL: "u2"}}
	plan := NewDeltaAnalyzer(st).Analyze("hash", repos)
	if plan.Decision != DeltaDecisionPartial || len(plan.ChangedRepos) != 1 || plan.ChangedRepos[0] != "u2" {
		t.Fatalf("expected partial with u2 changed, got %+v", plan)
	}
	if plan.RepoReasons["u1"] != RepoReasonUnchanged {
		t.Fatalf("expected u1 reason '%s' got %s", RepoReasonUnchanged, plan.RepoReasons["u1"])
	}
	if plan.RepoReasons["u2"] != RepoReasonUnknown {
		t.Fatalf("expected u2 reason '%s' got %s", RepoReasonUnknown, plan.RepoReasons["u2"])
	}
}

func TestDeltaAnalyzer_AllChangedFull(t *testing.T) {
	st := &fakeDeltaState{global: "g", perRepo: map[string]string{"u1": "", "u2": ""}, commits: map[string]string{"u1": "c1", "u2": ""}}
	repos := []cfg.Repository{{Name: "r1", URL: "u1"}, {Name: "r2", URL: "u2"}}
	plan := NewDeltaAnalyzer(st).Analyze("hash", repos)
	if plan.Decision != DeltaDecisionFull || (plan.Reason != "all_repos_changed" && plan.Reason != "all_repos_unknown_state") {
		t.Fatalf("expected full (all changed) got %+v", plan)
	}
	if plan.RepoReasons["u1"] != RepoReasonUnknown || plan.RepoReasons["u2"] != RepoReasonUnknown {
		t.Fatalf("expected unknown reasons got %+v", plan.RepoReasons)
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
		if err != nil || !fi.IsDir() {
			continue
		}
		if werr := filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			ln := strings.ToLower(d.Name())
			if strings.HasSuffix(ln, ".md") || strings.HasSuffix(ln, ".markdown") {
				rel, rerr := filepath.Rel(repoRoot, p)
				if rerr == nil {
					paths = append(paths, filepath.ToSlash(rel))
				}
			}
			return nil
		}); werr != nil {
			t.Fatalf("walkdir: %v", werr)
		}
	}
	if len(paths) == 0 {
		return ""
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Test single repo unchanged: quick hash matches stored -> no change (full rebuild with no_detected_repo_change)
func TestDeltaAnalyzer_QuickHashSingleRepoUnchanged(t *testing.T) {
	tmp := t.TempDir()
	repoName := "repo1"
	repoURL := "https://example.com/org/repo1.git"
	repoRoot := filepath.Join(tmp, repoName)
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "docs", "intro.md"), []byte("# Intro"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
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
	if err := os.MkdirAll(filepath.Join(repoARoot, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir repoA: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoARoot, "docs", "a1.md"), []byte("# A1"), 0o600); err != nil {
		t.Fatalf("write a1: %v", err)
	}
	hashA := computeQuickHash(t, repoARoot)
	// repo B (will change after snapshot)
	repoBName := "repoB"
	repoBURL := "https://example.com/org/repoB.git"
	repoBRoot := filepath.Join(tmp, repoBName)
	if err := os.MkdirAll(filepath.Join(repoBRoot, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir repoB: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBRoot, "docs", "b1.md"), []byte("# B1"), 0o600); err != nil {
		t.Fatalf("write b1: %v", err)
	}
	hashB := computeQuickHash(t, repoBRoot)
	if err := os.WriteFile(filepath.Join(repoBRoot, "docs", "b2.md"), []byte("# B2"), 0o600); err != nil {
		t.Fatalf("write b2: %v", err)
	}
	st := &fakeDeltaState{perRepo: map[string]string{repoAURL: hashA, repoBURL: hashB}, commits: map[string]string{repoAURL: "c1", repoBURL: "c2"}}
	repos := []cfg.Repository{{Name: repoAName, URL: repoAURL}, {Name: repoBName, URL: repoBURL}}
	plan := NewDeltaAnalyzer(st).WithWorkspace(tmp).Analyze("hash", repos)
	if plan.Decision != DeltaDecisionPartial || len(plan.ChangedRepos) != 1 || plan.ChangedRepos[0] != repoBURL {
		t.Fatalf("expected partial with repoB changed, got %+v", plan)
	}
}

// Verify RepoReasons population paths (unknown vs assumed_changed).
func TestDeltaAnalyzer_RepoReasons(t *testing.T) {
	st := &fakeDeltaState{perRepo: map[string]string{"u1": "h1", "u2": ""}, commits: map[string]string{"u1": "c1", "u2": ""}}
	repos := []cfg.Repository{{Name: "r1", URL: "u1"}, {Name: "r2", URL: "u2"}}
	plan := NewDeltaAnalyzer(st).Analyze("hash", repos)
	if plan.RepoReasons == nil {
		t.Fatalf("expected RepoReasons map")
	}
	if _, ok := plan.RepoReasons["u2"]; !ok {
		t.Fatalf("missing reason for u2")
	}
	if plan.RepoReasons["u2"] != RepoReasonUnknown {
		t.Fatalf("expected '%s' for u2 got %s", RepoReasonUnknown, plan.RepoReasons["u2"])
	}
	if plan.RepoReasons["u1"] == "" {
		t.Fatalf("expected non-empty reason for u1")
	}
}
