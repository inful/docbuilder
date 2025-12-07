package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"testing"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// hashPaths replicates the global/per-repo hashing (sorted paths, null separator) logic.
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

// TestPartialBuildRecomposesGlobalDocFilesHash ensures DeltaManager.RecomputeGlobalDocHash merges unchanged + changed repo paths.
func TestPartialBuildRecomposesGlobalDocFilesHash(t *testing.T) {
	workspace := t.TempDir()
	stateDir := filepath.Join(workspace, "state")
	svcResult := state.NewService(stateDir)
	if svcResult.IsErr() {
		t.Fatalf("state service: %v", svcResult.UnwrapErr())
	}
	sm := state.NewServiceAdapter(svcResult.Unwrap())

	repoAURL, repoAName := "https://example.com/org/repoA.git", "repoA"
	repoBURL, repoBName := "https://example.com/org/repoB.git", "repoB"
	repos := []cfg.Repository{{Name: repoAName, URL: repoAURL}, {Name: repoBName, URL: repoBURL}}
	sm.EnsureRepositoryState(repoAURL, repoAName, "")
	sm.EnsureRepositoryState(repoBURL, repoBName, "")

	// Seed initial full build state: one file per repo
	repoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md"))}
	repoBPaths := []string{filepath.ToSlash(filepath.Join(repoBName, "b1.md"))}
	sm.SetRepoDocFilePaths(repoAURL, repoAPaths)
	sm.SetRepoDocFilePaths(repoBURL, repoBPaths)
	sm.SetRepoDocFilesHash(repoAURL, hashPaths(repoAPaths))
	sm.SetRepoDocFilesHash(repoBURL, hashPaths(repoBPaths))
	globalFull := hashPaths(append(append([]string{}, repoAPaths...), repoBPaths...))
	sm.SetLastGlobalDocFilesHash(globalFull)

	// Simulate change: repoA adds a2.md (discovery this run will produce new repoA paths list)
	newRepoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md")), filepath.ToSlash(filepath.Join(repoAName, "a2.md"))}
	sm.SetRepoDocFilePaths(repoAURL, newRepoAPaths)
	sm.SetRepoDocFilesHash(repoAURL, hashPaths(newRepoAPaths))

	// Subset BuildReport (what generator would emit for changed repoA only) uses subset hash (only repoA paths)
	subsetHash := hashPaths(newRepoAPaths) // does not include repoB yet
	report := &hugo.BuildReport{DocFilesHash: subsetHash}

	// Build job with repositories metadata
	job := &BuildJob{
		TypedMeta: &BuildJobMetadata{Repositories: repos},
	}

	// Delta plan marking repoA changed
	deltaPlan := &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{repoAURL}}

	// Test DeltaManager.RecomputeGlobalDocHash directly
	dm := NewDeltaManager()
	deletions, err := dm.RecomputeGlobalDocHash(report, deltaPlan, sm, job, workspace, nil)
	if err != nil {
		t.Fatalf("RecomputeGlobalDocHash failed: %v", err)
	}
	if deletions != 0 {
		t.Errorf("expected 0 deletions, got %d", deletions)
	}

	if report.DocFilesHash == subsetHash {
		t.Fatalf("expected recomposed global hash different from subset hash: %s", subsetHash)
	}
	if report.DocFilesHash == globalFull {
		t.Fatalf("expected new global hash to differ from original full (new file added)")
	}
	if report.DocFilesHash == "" {
		t.Fatalf("recomposed hash empty")
	}
}

// TestPartialBuildDeletionNotReflectedYet documents current limitation: if a file is deleted
// in an unchanged repository, the recomposed global hash (after a partial build affecting
// a different repo) still includes the deleted file path because we rely on the persisted
// path list for unchanged repos until they are rebuilt or a full reconciliation occurs.
func TestPartialBuildDeletionNotReflectedYet(t *testing.T) {
	workspace := t.TempDir()
	stateDir := filepath.Join(workspace, "state")
	svcResult := state.NewService(stateDir)
	if svcResult.IsErr() {
		t.Fatalf("state service: %v", svcResult.UnwrapErr())
	}
	sm := state.NewServiceAdapter(svcResult.Unwrap())

	repoAURL, repoAName := "https://example.com/org/repoA.git", "repoA"
	repoBURL, repoBName := "https://example.com/org/repoB.git", "repoB"
	repos := []cfg.Repository{{Name: repoAName, URL: repoAURL}, {Name: repoBName, URL: repoBURL}}
	sm.EnsureRepositoryState(repoAURL, repoAName, "")
	sm.EnsureRepositoryState(repoBURL, repoBName, "")

	// Initial state: repoA: a1.md ; repoB: b1.md, b2.md
	repoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md"))}
	repoBPaths := []string{filepath.ToSlash(filepath.Join(repoBName, "b1.md")), filepath.ToSlash(filepath.Join(repoBName, "b2.md"))}
	sm.SetRepoDocFilePaths(repoAURL, repoAPaths)
	sm.SetRepoDocFilePaths(repoBURL, repoBPaths)
	sm.SetRepoDocFilesHash(repoAURL, hashPaths(repoAPaths))
	sm.SetRepoDocFilesHash(repoBURL, hashPaths(repoBPaths))
	globalFull := hashPaths(append(append([]string{}, repoAPaths...), repoBPaths...))
	sm.SetLastGlobalDocFilesHash(globalFull)

	// Simulate: repoA adds a2.md (causing partial build) and repoB deletes b2.md (not rebuilt this run)
	newRepoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md")), filepath.ToSlash(filepath.Join(repoAName, "a2.md"))}
	sm.SetRepoDocFilePaths(repoAURL, newRepoAPaths)
	sm.SetRepoDocFilesHash(repoAURL, hashPaths(newRepoAPaths))
	// IMPORTANT: we DO NOT update repoB path list (still includes b2.md) to reflect current limitation.

	subsetHash := hashPaths(newRepoAPaths) // what a changed-only subset would carry
	report := &hugo.BuildReport{DocFilesHash: subsetHash}

	job := &BuildJob{
		TypedMeta: &BuildJobMetadata{Repositories: repos},
	}

	// Delta plan marking repoA changed
	deltaPlan := &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{repoAURL}}

	// Test DeltaManager.RecomputeGlobalDocHash directly
	dm := NewDeltaManager()
	deletions, err := dm.RecomputeGlobalDocHash(report, deltaPlan, sm, job, workspace, nil)
	if err != nil {
		t.Fatalf("RecomputeGlobalDocHash failed: %v", err)
	}
	if deletions != 0 {
		t.Errorf("expected 0 deletions, got %d", deletions)
	}

	expectedWithDeletedStillPresent := hashPaths(append(append([]string{}, newRepoAPaths...), repoBPaths...)) // includes b2.md
	expectedIfDeletionHandled := hashPaths(append(append([]string{}, newRepoAPaths...), repoBPaths[:1]...))   // b2.md removed

	if report.DocFilesHash == subsetHash {
		t.Fatalf("recomposition did not occur (still subset hash)")
	}
	if report.DocFilesHash != expectedWithDeletedStillPresent {
		t.Fatalf("expected recomposed hash to still include deleted file path (current limitation). got=%s want=%s", report.DocFilesHash, expectedWithDeletedStillPresent)
	}
	if report.DocFilesHash == expectedIfDeletionHandled {
		t.Fatalf("deletion unexpectedly reflected; test must be updated to new behavior")
	}
	t.Logf("NOTE: deletion not reflected yet; recomposed hash includes stale path b2.md (expected current limitation)")
}
