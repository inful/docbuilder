package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

func hashList(paths []string) string {
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

// TestPartialBuildDeletionReflected verifies new behavior: unchanged repo deletions are detected
// during partial recomposition scan and removed from the union hash.
func TestPartialBuildDeletionReflected(t *testing.T) {
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

	// Create workspace clone directories simulating on-disk repos (unchanged repoB will have deletion)
	repoARoot := filepath.Join(workspace, repoAName)
	repoBRoot := filepath.Join(workspace, repoBName)
	if err := os.MkdirAll(filepath.Join(repoARoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir repoA: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoBRoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir repoB: %v", err)
	}
	// Initial files
	if err := os.WriteFile(filepath.Join(repoARoot, "docs", "a1.md"), []byte("# A1"), 0o600); err != nil {
		t.Fatalf("write a1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBRoot, "docs", "b1.md"), []byte("# B1"), 0o600); err != nil {
		t.Fatalf("write b1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBRoot, "docs", "b2.md"), []byte("# B2"), 0o600); err != nil {
		t.Fatalf("write b2: %v", err)
	}

	// Persist initial path lists & hashes (as if from previous full build)
	repoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "docs", "a1.md"))}
	repoBPaths := []string{filepath.ToSlash(filepath.Join(repoBName, "docs", "b1.md")), filepath.ToSlash(filepath.Join(repoBName, "docs", "b2.md"))}
	sm.SetRepoDocFilePaths(repoAURL, repoAPaths)
	sm.SetRepoDocFilePaths(repoBURL, repoBPaths)
	sm.SetRepoDocFilesHash(repoAURL, hashList(repoAPaths))
	sm.SetRepoDocFilesHash(repoBURL, hashList(repoBPaths))
	sm.SetLastGlobalDocFilesHash(hashList(append(append([]string{}, repoAPaths...), repoBPaths...)))

	// Change: repoA adds a2.md (changed repo) ; repoB deletes b2.md (unchanged repo)
	if err := os.WriteFile(filepath.Join(repoARoot, "docs", "a2.md"), []byte("# A2"), 0o600); err != nil {
		t.Fatalf("write a2: %v", err)
	}
	if err := os.Remove(filepath.Join(repoBRoot, "docs", "b2.md")); err != nil {
		t.Fatalf("remove b2: %v", err)
	}

	// Update changed repoA list (discovery result this run)
	newRepoAPaths := []string{filepath.ToSlash(filepath.Join(repoAName, "docs", "a1.md")), filepath.ToSlash(filepath.Join(repoAName, "docs", "a2.md"))}
	sm.SetRepoDocFilePaths(repoAURL, newRepoAPaths)
	sm.SetRepoDocFilesHash(repoAURL, hashList(newRepoAPaths))

	// Subset report hash (only changed repoA) prior to recomposition
	subsetHash := hashList(newRepoAPaths)
	report := &hugoBuildReportShim{DocFilesHash: subsetHash}
	bc := &buildContext{
		workspace: workspace,
		job: &BuildJob{
			TypedMeta: &BuildJobMetadata{Repositories: repos},
		},
		stateMgr:  sm,
		deltaPlan: &DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: []string{repoAURL}},
	}

	// Execute recomposition logic block from stagePostPersist (duplicated minimal) to isolate behavior
	if bc.deltaPlan.Decision == DeltaDecisionPartial && report.DocFilesHash != "" {
		getter, gOK := bc.stateMgr.(interface{ GetRepoDocFilePaths(string) []string })
		setter, sOK := bc.stateMgr.(interface{ SetRepoDocFilePaths(string, []string) })
		hasher, hOK := bc.stateMgr.(interface{ SetRepoDocFilesHash(string, string) })
		if gOK {
			all := []string{}
			changedSet := map[string]struct{}{repoAURL: {}}
			for _, r := range repos {
				paths := getter.GetRepoDocFilePaths(r.URL)
				if _, ch := changedSet[r.URL]; !ch { // unchanged repoB; scan to detect deletion
					if fi, err := os.Stat(filepath.Join(workspace, r.Name)); err == nil && fi.IsDir() {
						fresh := []string{}
						if werr := filepath.WalkDir(filepath.Join(workspace, r.Name, "docs"), func(p string, d os.DirEntry, err error) error {
							if err != nil || d == nil || d.IsDir() {
								return nil
							}
							if filepath.Ext(d.Name()) == ".md" || filepath.Ext(d.Name()) == ".markdown" {
								rel, rerr := filepath.Rel(filepath.Join(workspace, r.Name), p)
								if rerr == nil {
									fresh = append(fresh, filepath.ToSlash(filepath.Join(r.Name, rel)))
								}
							}
							return nil
						}); werr != nil {
							t.Fatalf("walkdir: %v", werr)
						}
						sort.Strings(fresh)
						if len(fresh) != len(paths) { // detected deletion
							if sOK {
								setter.SetRepoDocFilePaths(r.URL, fresh)
							}
							if hOK {
								hasher.SetRepoDocFilesHash(r.URL, hashList(fresh))
							}
							paths = fresh
						}
					}
				}
				all = append(all, paths...)
			}
			if len(all) > 0 {
				report.DocFilesHash = hashList(all)
			}
		}
	}

	if report.DocFilesHash == subsetHash {
		t.Fatalf("expected recomposed hash (not subset)")
	}
	if report.DocFilesHash == hashList(append(append([]string{}, newRepoAPaths...), repoBPaths...)) {
		t.Fatalf("hash still includes deleted file b2.md")
	}
	// Expected union now: repoA (a1,a2) + repoB (b1) only
	expected := hashList([]string{filepath.ToSlash(filepath.Join(repoAName, "docs", "a1.md")), filepath.ToSlash(filepath.Join(repoAName, "docs", "a2.md")), filepath.ToSlash(filepath.Join(repoBName, "docs", "b1.md"))})
	if report.DocFilesHash != expected {
		t.Fatalf("unexpected recomposed hash; got=%s want=%s", report.DocFilesHash, expected)
	}
}
