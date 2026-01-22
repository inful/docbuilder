package delta

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

const (
	repoAURL  = "https://example.com/org/repoA.git"
	repoBURL  = "https://example.com/org/repoB.git"
	repoAName = "repoA"
	repoBName = "repoB"
)

func hashPaths(paths []string) string {
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

func TestManager_AttachDeltaMetadata_RepoReasonsPropagation(t *testing.T) {
	report := &models.BuildReport{}
	deltaPlan := &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{"u1", "u2"}}
	m := NewManager()
	m.AttachDeltaMetadata(report, deltaPlan, map[string]string{"u1": RepoReasonUnknown, "u2": RepoReasonQuickHashDiff})

	if len(report.DeltaRepoReasons) != 2 {
		t.Fatalf("expected 2 repo reasons, got %d", len(report.DeltaRepoReasons))
	}
	if report.DeltaRepoReasons["u1"] != RepoReasonUnknown {
		t.Fatalf("u1 reason mismatch: got %s", report.DeltaRepoReasons["u1"])
	}
	if report.DeltaRepoReasons["u2"] != RepoReasonQuickHashDiff {
		t.Fatalf("u2 reason mismatch: got %s", report.DeltaRepoReasons["u2"])
	}
}

func TestManager_RecomputeGlobalDocHash_RecomposesUnion(t *testing.T) {
	workspace := t.TempDir()
	stateDir := filepath.Join(workspace, "state")
	svcResult := state.NewService(stateDir)
	if svcResult.IsErr() {
		t.Fatalf("state service: %v", svcResult.UnwrapErr())
	}
	meta := state.NewServiceAdapter(svcResult.Unwrap())

	repos := []cfg.Repository{{Name: repoAName, URL: repoAURL}, {Name: repoBName, URL: repoBURL}}
	meta.EnsureRepositoryState(repoAURL, repoAName, "")
	meta.EnsureRepositoryState(repoBURL, repoBName, "")

	repoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md"))}
	repoBPaths := []string{filepath.ToSlash(filepath.Join(repoBName, "b1.md"))}
	meta.SetRepoDocFilePaths(repoAURL, repoAPaths)
	meta.SetRepoDocFilePaths(repoBURL, repoBPaths)
	meta.SetRepoDocFilesHash(repoAURL, hashPaths(repoAPaths))
	meta.SetRepoDocFilesHash(repoBURL, hashPaths(repoBPaths))
	globalFull := hashPaths(append(append([]string{}, repoAPaths...), repoBPaths...))
	meta.SetLastGlobalDocFilesHash(globalFull)

	newRepoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md")), filepath.ToSlash(filepath.Join(repoAName, "a2.md"))}
	meta.SetRepoDocFilePaths(repoAURL, newRepoAPaths)
	meta.SetRepoDocFilesHash(repoAURL, hashPaths(newRepoAPaths))

	subsetHash := hashPaths(newRepoAPaths)
	report := &models.BuildReport{DocFilesHash: subsetHash}
	plan := &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{repoAURL}}

	m := NewManager()
	deletions, err := m.RecomputeGlobalDocHash(report, plan, meta, repos, workspace, nil)
	if err != nil {
		t.Fatalf("RecomputeGlobalDocHash failed: %v", err)
	}
	if deletions != 0 {
		t.Fatalf("expected 0 deletions, got %d", deletions)
	}
	if report.DocFilesHash == subsetHash {
		t.Fatalf("expected recomposed global hash different from subset hash: %s", subsetHash)
	}
	if report.DocFilesHash == "" {
		t.Fatalf("recomposed hash empty")
	}
}

func TestManager_RecomputeGlobalDocHash_DetectsDeletionsInUnchangedRepo(t *testing.T) {
	workspace := t.TempDir()
	stateDir := filepath.Join(workspace, "state")
	svcResult := state.NewService(stateDir)
	if svcResult.IsErr() {
		t.Fatalf("state service: %v", svcResult.UnwrapErr())
	}
	meta := state.NewServiceAdapter(svcResult.Unwrap())

	repos := []cfg.Repository{{Name: repoAName, URL: repoAURL}, {Name: repoBName, URL: repoBURL}}
	meta.EnsureRepositoryState(repoAURL, repoAName, "")
	meta.EnsureRepositoryState(repoBURL, repoBName, "")

	repoARoot := filepath.Join(workspace, repoAName)
	repoBRoot := filepath.Join(workspace, repoBName)
	if err := os.MkdirAll(filepath.Join(repoARoot, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir repoA: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoBRoot, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir repoB: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoARoot, "docs", "a1.md"), []byte("# A1"), 0o600); err != nil {
		t.Fatalf("write a1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBRoot, "docs", "b1.md"), []byte("# B1"), 0o600); err != nil {
		t.Fatalf("write b1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBRoot, "docs", "b2.md"), []byte("# B2"), 0o600); err != nil {
		t.Fatalf("write b2: %v", err)
	}

	repoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "docs", "a1.md"))}
	repoBPaths := []string{filepath.ToSlash(filepath.Join(repoBName, "docs", "b1.md")), filepath.ToSlash(filepath.Join(repoBName, "docs", "b2.md"))}
	meta.SetRepoDocFilePaths(repoAURL, repoAPaths)
	meta.SetRepoDocFilePaths(repoBURL, repoBPaths)
	meta.SetRepoDocFilesHash(repoAURL, hashPaths(repoAPaths))
	meta.SetRepoDocFilesHash(repoBURL, hashPaths(repoBPaths))
	meta.SetLastGlobalDocFilesHash(hashPaths(append(append([]string{}, repoAPaths...), repoBPaths...)))

	if err := os.WriteFile(filepath.Join(repoARoot, "docs", "a2.md"), []byte("# A2"), 0o600); err != nil {
		t.Fatalf("write a2: %v", err)
	}
	if err := os.Remove(filepath.Join(repoBRoot, "docs", "b2.md")); err != nil {
		t.Fatalf("remove b2: %v", err)
	}

	newRepoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "docs", "a1.md")), filepath.ToSlash(filepath.Join(repoAName, "docs", "a2.md"))}
	meta.SetRepoDocFilePaths(repoAURL, newRepoAPaths)
	meta.SetRepoDocFilesHash(repoAURL, hashPaths(newRepoAPaths))

	subsetHash := hashPaths(newRepoAPaths)
	report := &models.BuildReport{DocFilesHash: subsetHash}
	plan := &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{repoAURL}}

	buildCfg := &cfg.Config{Build: cfg.BuildConfig{DetectDeletions: true}}
	m := NewManager()
	deletions, err := m.RecomputeGlobalDocHash(report, plan, meta, repos, workspace, buildCfg)
	if err != nil {
		t.Fatalf("RecomputeGlobalDocHash failed: %v", err)
	}
	if deletions != 1 {
		t.Fatalf("expected 1 deletion detected, got %d", deletions)
	}
	if report.DocFilesHash == subsetHash {
		t.Fatalf("expected recomposed hash (not subset)")
	}
	if report.DocFilesHash == hashPaths(append(append([]string{}, newRepoAPaths...), repoBPaths...)) {
		t.Fatalf("hash still includes deleted file b2.md")
	}
	expected := hashPaths([]string{
		filepath.ToSlash(filepath.Join(repoAName, "docs", "a1.md")),
		filepath.ToSlash(filepath.Join(repoAName, "docs", "a2.md")),
		filepath.ToSlash(filepath.Join(repoBName, "docs", "b1.md")),
	})
	if report.DocFilesHash != expected {
		t.Fatalf("unexpected recomposed hash; got=%s want=%s", report.DocFilesHash, expected)
	}
}
