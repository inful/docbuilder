package git

import (
    "errors"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"

    appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
    "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing"
    "github.com/go-git/go-git/v5/plumbing/object"
    ggitcfg "github.com/go-git/go-git/v5/config"
)

// TestComputeBackoffDelay verifies backoff strategies and max capping.
func TestComputeBackoffDelay(t *testing.T) {
    init := 1 * time.Millisecond
    max := 10 * time.Millisecond

    // Fixed
    if d := computeBackoffDelay("fixed", 0, init, max); d != init { t.Fatalf("fixed attempt0 expected %v got %v", init, d) }
    if d := computeBackoffDelay("fixed", 5, init, max); d != init { t.Fatalf("fixed attempt5 expected %v got %v", init, d) }

    // Linear: (attempt+1)*init
    if d := computeBackoffDelay("linear", 0, init, max); d != init { t.Fatalf("linear attempt0 expected %v got %v", init, d) }
    if d := computeBackoffDelay("linear", 3, init, max); d != 4*init { t.Fatalf("linear attempt3 expected %v got %v", 4*init, d) }

    // Exponential: init * 2^attempt
    if d := computeBackoffDelay("exponential", 0, init, max); d != init { t.Fatalf("exp attempt0 expected %v got %v", init, d) }
    if d := computeBackoffDelay("exponential", 1, init, max); d != 2*init { t.Fatalf("exp attempt1 expected %v got %v", 2*init, d) }
    // attempt 5 -> 32*init but capped at max
    if d := computeBackoffDelay("exponential", 5, init, max); d != max { t.Fatalf("exp attempt5 expected cap %v got %v", max, d) }
}

// TestWithRetryBehavior ensures retries happen for transient errors and stop for permanent ones.
func TestWithRetryBehavior(t *testing.T) {
    cfg := &appcfg.BuildConfig{MaxRetries: 3, RetryBackoff: appcfg.RetryBackoffFixed, RetryInitialDelay: "1ms", RetryMaxDelay: "5ms"}
    c := NewClient(t.TempDir()).WithBuildConfig(cfg)

    attempts := 0
    // Transient failure first 2 attempts, then success
    path, err := c.withRetry("clone", "repo", func() (string, error) {
        if attempts < 2 { attempts++; return "", errors.New("temporary network failure") }
        attempts++
        return "/ok", nil
    })
    if err != nil { t.Fatalf("expected success transient scenario: %v", err) }
    if attempts != 3 { t.Fatalf("expected 3 attempts got %d", attempts) }
    if path != "/ok" { t.Fatalf("unexpected path %s", path) }

    // Permanent error should not retry
    attempts = 0
    _, err = c.withRetry("clone", "repo", func() (string, error) {
        attempts++
        return "", errors.New("authentication failed: permission denied")
    })
    if err == nil { t.Fatalf("expected permanent error") }
    if attempts != 1 { t.Fatalf("expected 1 attempt for permanent error, got %d", attempts) }
}

// TestIsPermanentGitError basic classification sanity.
func TestIsPermanentGitError(t *testing.T) {
    if !isPermanentGitError(errors.New("authentication failed")) { t.Fatalf("expected auth classified permanent") }
    if isPermanentGitError(errors.New("temporary network failure")) { t.Fatalf("expected temporary network error not permanent") }
}

// helper to add a file and commit returning hash.
func addFileAndCommit(repo *git.Repository, repoPath, filename, content, msg string) (plumbing.Hash, error) {
    wt, err := repo.Worktree()
    if err != nil { return plumbing.Hash{}, err }
    full := filepath.Join(repoPath, filename)
    if err := os.WriteFile(full, []byte(content), 0o644); err != nil { return plumbing.Hash{}, err }
    if _, err := wt.Add(filename); err != nil { return plumbing.Hash{}, err }
    hash, err := wt.Commit(msg, &git.CommitOptions{Author: &object.Signature{Name: "tester", Email: "t@example.com", When: time.Now()}})
    if err != nil { return plumbing.Hash{}, err }
    return hash, nil
}

// TestDivergenceHandling verifies divergence error vs hard reset behavior.
func TestDivergenceHandling(t *testing.T) {
    tmp := t.TempDir()
    barePath := filepath.Join(tmp, "remote.git")
    if _, err := git.PlainInit(barePath, true); err != nil { t.Fatalf("init bare: %v", err) }

    // Working repo to seed remote
    workPath := filepath.Join(tmp, "seed")
    workRepo, err := git.PlainInit(workPath, false)
    if err != nil { t.Fatalf("init work: %v", err) }
    // Add bare as origin
    if _, err := workRepo.CreateRemote(&ggitcfg.RemoteConfig{Name: "origin", URLs: []string{barePath}}); err != nil { t.Fatalf("create remote: %v", err) }

    if _, err := addFileAndCommit(workRepo, workPath, "a.txt", "A", "A"); err != nil { t.Fatalf("commit A: %v", err) }
    if err := workRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil { t.Fatalf("push A: %v", err) }

    // Clone to local workspace repo (will become diverging later)
    ws := filepath.Join(tmp, "ws")
    if err := os.MkdirAll(ws, 0o755); err != nil { t.Fatalf("mkdir ws: %v", err) }
    localPath := filepath.Join(ws, "repo")
    if _, err := git.PlainClone(localPath, false, &git.CloneOptions{URL: barePath, ReferenceName: plumbing.NewBranchReferenceName("master"), SingleBranch: true}); err != nil { t.Fatalf("clone local: %v", err) }

    // Create commit B locally (diverging)
    localRepo, err := git.PlainOpen(localPath)
    if err != nil { t.Fatalf("open local: %v", err) }
    if _, err := addFileAndCommit(localRepo, localPath, "b.txt", "B", "B"); err != nil { t.Fatalf("commit B: %v", err) }

    // Create commit C in remote working repo (still pointing to parent A) and push
    if _, err := addFileAndCommit(workRepo, workPath, "c.txt", "C", "C"); err != nil { t.Fatalf("commit C: %v", err) }
    if err := workRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil { t.Fatalf("push C: %v", err) }

    repoCfg := appcfg.Repository{Name: "repo", URL: barePath, Branch: "master"}

    // Case 1: HardResetOnDiverge = false -> expect divergence error
    client := NewClient(ws).WithBuildConfig(&appcfg.BuildConfig{HardResetOnDiverge: false})
    if _, err := client.updateExistingRepo(localPath, repoCfg); err == nil || !strings.Contains(err.Error(), "diverged") {
        t.Fatalf("expected divergence error, got %v", err)
    }

    // Capture remote hash via local remote tracking ref before applying hard reset
    localRemoteRefBefore, err := localRepo.Reference(plumbing.NewRemoteReferenceName("origin", "master"), true)
    if err != nil { t.Fatalf("expected remote ref before: %v", err) }
    remoteHash := localRemoteRefBefore.Hash()

    // Case 2: HardResetOnDiverge = true -> expect success and head equals remote hash
    client2 := NewClient(ws).WithBuildConfig(&appcfg.BuildConfig{HardResetOnDiverge: true})
    if _, err := client2.updateExistingRepo(localPath, repoCfg); err != nil { t.Fatalf("expected hard reset success: %v", err) }
    updatedRepo, _ := git.PlainOpen(localPath)
    head, _ := updatedRepo.Head()
    if head.Hash() != remoteHash { t.Fatalf("expected local head %s to equal remote %s", head.Hash(), remoteHash) }
}

// (removed custom contains helpers; using strings.Contains instead)
