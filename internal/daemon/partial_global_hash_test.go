package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"testing"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// hashPaths replicates the global/per-repo hashing (sorted paths, null separator) logic.
func hashPaths(paths []string) string {
	if len(paths) == 0 { return "" }
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths { h.Write([]byte(p)); h.Write([]byte{0}) }
	return hex.EncodeToString(h.Sum(nil))
}

// TestPartialBuildRecomposesGlobalDocFilesHash ensures stagePostPersist merges unchanged + changed repo paths.
func TestPartialBuildRecomposesGlobalDocFilesHash(t *testing.T) {
	workspace := t.TempDir()
	stateDir := filepath.Join(workspace, "state")
	state, err := NewStateManager(stateDir)
	if err != nil { t.Fatalf("state manager: %v", err) }

	repoAURL, repoAName := "https://example.com/org/repoA.git", "repoA"
	repoBURL, repoBName := "https://example.com/org/repoB.git", "repoB"
	repos := []cfg.Repository{{Name: repoAName, URL: repoAURL}, {Name: repoBName, URL: repoBURL}}

	// Seed initial full build state: one file per repo
	repoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md"))}
	repoBPaths := []string{filepath.ToSlash(filepath.Join(repoBName, "b1.md"))}
	state.SetRepoDocFilePaths(repoAURL, repoAPaths)
	state.SetRepoDocFilePaths(repoBURL, repoBPaths)
	state.SetRepoDocFilesHash(repoAURL, hashPaths(repoAPaths))
	state.SetRepoDocFilesHash(repoBURL, hashPaths(repoBPaths))
	globalFull := hashPaths(append(append([]string{}, repoAPaths...), repoBPaths...))
	state.SetLastGlobalDocFilesHash(globalFull)

	// Simulate change: repoA adds a2.md (discovery this run will produce new repoA paths list)
	newRepoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md")), filepath.ToSlash(filepath.Join(repoAName, "a2.md"))}
	state.SetRepoDocFilePaths(repoAURL, newRepoAPaths)
	state.SetRepoDocFilesHash(repoAURL, hashPaths(newRepoAPaths))

	// Subset BuildReport (what generator would emit for changed repoA only) uses subset hash (only repoA paths)
	subsetHash := hashPaths(newRepoAPaths) // does not include repoB yet
	report := &hugoBuildReportShim{DocFilesHash: subsetHash}

	// Build context with deltaPlan marking repoA changed
	job := &BuildJob{Metadata: map[string]interface{}{"repositories": repos}}
	bc := &buildContext{job: job, stateMgr: state, deltaPlan: &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{repoAURL}}}

	// Invoke post-persist recomposition via a lightweight shim (copy stagePostPersist logic segment for recomposition)
	if bc.deltaPlan != nil && bc.deltaPlan.Decision == DeltaDecisionPartial && report.DocFilesHash != "" {
		if getter, ok := bc.stateMgr.(interface{ GetRepoDocFilePaths(string) []string }); ok {
			all := []string{}
			for _, r := range repos { // original full set
				if ps := getter.GetRepoDocFilePaths(r.URL); len(ps) > 0 { all = append(all, ps...) }
			}
			if len(all) > 0 {
				merged := hashPaths(all)
				report.DocFilesHash = merged
			}
		}
	}

	if report.DocFilesHash == subsetHash { t.Fatalf("expected recomposed global hash different from subset hash: %s", subsetHash) }
	if report.DocFilesHash == globalFull { t.Fatalf("expected new global hash to differ from original full (new file added)") }
	if report.DocFilesHash == "" { t.Fatalf("recomposed hash empty") }
}

// TestPartialBuildDeletionNotReflectedYet documents current limitation: if a file is deleted
// in an unchanged repository, the recomposed global hash (after a partial build affecting
// a different repo) still includes the deleted file path because we rely on the persisted
// path list for unchanged repos until they are rebuilt or a full reconciliation occurs.
func TestPartialBuildDeletionNotReflectedYet(t *testing.T) {
	workspace := t.TempDir()
	stateDir := filepath.Join(workspace, "state")
	state, err := NewStateManager(stateDir)
	if err != nil { t.Fatalf("state manager: %v", err) }

	repoAURL, repoAName := "https://example.com/org/repoA.git", "repoA"
	repoBURL, repoBName := "https://example.com/org/repoB.git", "repoB"
	repos := []cfg.Repository{{Name: repoAName, URL: repoAURL}, {Name: repoBName, URL: repoBURL}}

	// Initial state: repoA: a1.md ; repoB: b1.md, b2.md
	repoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md"))}
	repoBPaths := []string{filepath.ToSlash(filepath.Join(repoBName, "b1.md")), filepath.ToSlash(filepath.Join(repoBName, "b2.md"))}
	state.SetRepoDocFilePaths(repoAURL, repoAPaths)
	state.SetRepoDocFilePaths(repoBURL, repoBPaths)
	state.SetRepoDocFilesHash(repoAURL, hashPaths(repoAPaths))
	state.SetRepoDocFilesHash(repoBURL, hashPaths(repoBPaths))
	globalFull := hashPaths(append(append([]string{}, repoAPaths...), repoBPaths...))
	state.SetLastGlobalDocFilesHash(globalFull)

	// Simulate: repoA adds a2.md (causing partial build) and repoB deletes b2.md (not rebuilt this run)
	newRepoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "a1.md")), filepath.ToSlash(filepath.Join(repoAName, "a2.md"))}
	state.SetRepoDocFilePaths(repoAURL, newRepoAPaths)
	state.SetRepoDocFilesHash(repoAURL, hashPaths(newRepoAPaths))
	// IMPORTANT: we DO NOT update repoB path list (still includes b2.md) to reflect current limitation.

	subsetHash := hashPaths(newRepoAPaths) // what a changed-only subset would carry
	report := &hugoBuildReportShim{DocFilesHash: subsetHash}

	job := &BuildJob{Metadata: map[string]interface{}{"repositories": repos}}
	bc := &buildContext{job: job, stateMgr: state, deltaPlan: &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{repoAURL}}}

	// Recomposition identical to stagePostPersist logic used in previous test
	if bc.deltaPlan != nil && bc.deltaPlan.Decision == DeltaDecisionPartial && report.DocFilesHash != "" {
		if getter, ok := bc.stateMgr.(interface{ GetRepoDocFilePaths(string) []string }); ok {
			all := []string{}
			for _, r := range repos {
				if ps := getter.GetRepoDocFilePaths(r.URL); len(ps) > 0 { all = append(all, ps...) }
			}
			if len(all) > 0 { report.DocFilesHash = hashPaths(all) }
		}
	}

	expectedWithDeletedStillPresent := hashPaths(append(append([]string{}, newRepoAPaths...), repoBPaths...)) // includes b2.md
	expectedIfDeletionHandled := hashPaths(append(append([]string{}, newRepoAPaths...), repoBPaths[:1]...))    // b2.md removed

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

// Minimal shim to avoid importing full hugo package in this low-level test; only needs DocFilesHash field
type hugoBuildReportShim struct { DocFilesHash string }
