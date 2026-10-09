package git

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// TestUpdateRepoSelfHealsOnPackCorruption asserts that a workspace whose pack file
// has been corrupted (e.g. by an interrupted fetch, partial write, or
// filesystem fault) is transparently recovered on the next UpdateRepo call,
// rather than surfacing
// "[git:error] failed to checkout existing branch: invalid checksum" to the user.
//
// Repro shape: bare remote with a few commits -> clone to workspace -> corrupt
// the workspace's pack file -> push one more commit on the remote -> UpdateRepo.
// The checkout of the new branch is forced to read the corrupt pack (it must
// resolve the commit's tree, which lives there) and fails integrity checks.
// Before the self-heal fix this returns the wrapped go-git error; with the fix
// the workspace is wiped and re-cloned, and the update succeeds.
func TestUpdateRepoSelfHealsOnPackCorruption(t *testing.T) {
	tmp := t.TempDir()

	// Bare "remote" we'll seed and push to.
	barePath := filepath.Join(tmp, "remote.git")
	seedPath := filepath.Join(tmp, "seed")
	ws := filepath.Join(tmp, "ws")
	repoPath := filepath.Join(ws, "repo")
	author := &object.Signature{Name: "tester", Email: "t@example.com", When: time.Now()}

	if _, err := git.PlainInit(barePath, true); err != nil {
		t.Fatalf("init bare: %v", err)
	}
	seedRepo, err := git.PlainInit(seedPath, false)
	if err != nil {
		t.Fatalf("init seed: %v", err)
	}
	if _, err = seedRepo.CreateRemote(&ggitcfg.RemoteConfig{Name: "origin", URLs: []string{barePath}}); err != nil {
		t.Fatalf("create remote: %v", err)
	}

	seedWt, err := seedRepo.Worktree()
	if err != nil {
		t.Fatalf("seed worktree: %v", err)
	}

	// Two commits on the remote so the initial workspace pack has real
	// content. The first commit adds a large-ish file (so it doesn't get
	// inlined as a delta by go-git) and the second changes a different file
	// (so the second commit's tree must reference the first commit's tree
	// when go-git computes the worktree diff).
	if err = os.WriteFile(filepath.Join(seedPath, "blob.bin"), make([]byte, 4096), 0o600); err != nil {
		t.Fatalf("write blob.bin: %v", err)
	}
	if _, err = seedWt.Add("blob.bin"); err != nil {
		t.Fatalf("add blob.bin: %v", err)
	}
	if _, err = seedWt.Commit("A", &git.CommitOptions{Author: author}); err != nil {
		t.Fatalf("commit A: %v", err)
	}
	if err = os.WriteFile(filepath.Join(seedPath, "other.txt"), []byte("other"), 0o600); err != nil {
		t.Fatalf("write other.txt: %v", err)
	}
	if _, err = seedWt.Add("other.txt"); err != nil {
		t.Fatalf("add other.txt: %v", err)
	}
	if _, err = seedWt.Commit("B", &git.CommitOptions{Author: author}); err != nil {
		t.Fatalf("commit B: %v", err)
	}
	if err = seedRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil {
		t.Fatalf("push: %v", err)
	}

	if err = os.MkdirAll(ws, 0o750); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	if _, err = git.PlainClone(repoPath, false, &git.CloneOptions{
		URL:           barePath,
		ReferenceName: plumbing.NewBranchReferenceName("master"),
		SingleBranch:  true,
	}); err != nil {
		t.Fatalf("initial clone: %v", err)
	}

	// Corrupt every pack's .idx in the workspace. The .idx checksum IS
	// validated on every pack access (when go-git lazily loads the index
	// for the pack), so this deterministically breaks the next object
	// read. We corrupt all of them so the test doesn't depend on which
	// pack go-git happens to read from.
	idxs, err := filepath.Glob(filepath.Join(repoPath, ".git", "objects", "pack", "*.idx"))
	if err != nil {
		t.Fatalf("glob idxs: %v", err)
	}
	if len(idxs) == 0 {
		t.Fatalf("expected at least one pack idx in workspace")
	}
	for _, p := range idxs {
		if err = corruptPackIdx(p); err != nil {
			t.Fatalf("corrupt idx %s: %v", p, err)
		}
	}

	// Push a third commit on the remote so the next update must do real
	// work (fetch + checkout) and the checkout will read the corrupt pack.
	if err = os.WriteFile(filepath.Join(seedPath, "third.txt"), []byte("third"), 0o600); err != nil {
		t.Fatalf("write third.txt: %v", err)
	}
	if _, err = seedWt.Add("third.txt"); err != nil {
		t.Fatalf("add third.txt: %v", err)
	}
	if _, err = seedWt.Commit("C", &git.CommitOptions{Author: author}); err != nil {
		t.Fatalf("commit C: %v", err)
	}
	if err = seedRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil {
		t.Fatalf("push C: %v", err)
	}
	expectedHead, err := seedRepo.Head()
	if err != nil {
		t.Fatalf("seed head: %v", err)
	}

	// Sanity check: the corruption is real. Reading a blob from the
	// workspace should now fail (we verify this in a separate repo open so
	// the test fails fast and clearly if the corruption method ever stops
	// being effective).
	verifyCorruptionTriggers(t, repoPath)

	// Run the docbuilder update path. Before the self-heal fix this returns
	// the wrapped go-git checksum error; with the fix it transparently
	// wipes the corrupt workspace and re-clones.
	cfg := &appcfg.BuildConfig{
		MaxRetries:        1,
		RetryBackoff:      appcfg.RetryBackoffFixed,
		RetryInitialDelay: "1ms",
		RetryMaxDelay:     "5ms",
	}
	client := NewClient(ws).WithBuildConfig(cfg)
	repoCfg := appcfg.Repository{Name: "repo", URL: barePath, Branch: "master"}
	path, err := client.UpdateRepo(repoCfg)
	if err != nil {
		t.Fatalf("expected self-heal to recover from pack corruption, got: %v", err)
	}
	if path != repoPath {
		t.Fatalf("expected path %s, got %s", repoPath, path)
	}

	finalRepo, err := git.PlainOpen(path)
	if err != nil {
		t.Fatalf("open recovered repo: %v", err)
	}
	head, err := finalRepo.Head()
	if err != nil {
		t.Fatalf("read recovered HEAD: %v", err)
	}
	if head.Hash() != expectedHead.Hash() {
		t.Fatalf("recovered HEAD %s != expected %s", head.Hash(), expectedHead.Hash())
	}
	if _, err = os.Stat(filepath.Join(path, "third.txt")); err != nil {
		t.Fatalf("expected third.txt to be checked out after recovery: %v", err)
	}
}

// verifyCorruptionTriggers opens the workspace and tries to read the commit
// pointed at by HEAD. With a corrupt pack this should fail; if it doesn't
// (because go-git cached the object in memory during PlainOpen, or because
// our corruption method was a no-op) the test setup is wrong and we should
// fail loudly here rather than letting the main test pass for the wrong
// reason.
func verifyCorruptionTriggers(t *testing.T, repoPath string) {
	t.Helper()
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		// PlainOpen may itself fail on a corrupt pack — that's fine and
		// proves the corruption is real.
		return
	}
	head, err := r.Head()
	if err != nil {
		return
	}
	_, err = r.CommitObject(head.Hash())
	if err == nil {
		t.Fatalf("corruption setup did not actually break object reads; check corruptPackIdx")
	}
}

// TestClassifyGitErrorMarksCorruptionAsRetryable ensures the go-git pack / idx
// decoder errors are surfaced as retryable + flagged with corruption=true so
// the self-heal path can take over.
func TestClassifyGitErrorMarksCorruptionAsRetryable(t *testing.T) {
	cases := []struct {
		name string
		msg  string
	}{
		{"invalid checksum", "invalid checksum"},
		{"idx checksum mismatch", "malformed IDX file: checksum mismatch: \"00\" instead of \"01\""},
		{"malformed pack", "malformed PACK file"},
		{"empty pack", "empty packfile"},
		{"packfile mismatch", "malformed IDX file: packfile mismatch: target is \"x\" not \"y\""},
		{"object not found from corrupt idx", "object not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ClassifyGitError(errors.New(tc.msg), "checkout", "https://example.com/repo.git")
			if err == nil {
				t.Fatalf("expected classified error")
			}
			if !isCorruptionError(err) {
				t.Fatalf("expected corruption=true for %q, got err=%v", tc.msg, err)
			}
			if isPermanentGitError(err) {
				t.Fatalf("corruption errors must not be permanent (would block self-heal retry): %v", err)
			}
		})
	}
}

// TestIsCorruptionErrorKeepsNotFoundForRepository ensures the corruption case
// in ClassifyGitError doesn't swallow the legitimate "repository not found"
// classification — that's a real remote problem, not local corruption, and
// must stay in CategoryNotFound.
func TestIsCorruptionErrorKeepsNotFoundForRepository(t *testing.T) {
	err := ClassifyGitError(errors.New("repository not found"), "fetch", "https://example.com/missing.git")
	if isCorruptionError(err) {
		t.Fatalf("repository not found must not be classified as corruption, got %v", err)
	}
	ce, ok := derrors.AsClassified(err)
	if !ok {
		t.Fatalf("expected classified error")
	}
	if !ce.IsCategory(derrors.CategoryNotFound) {
		t.Fatalf("expected category not_found, got %s", ce.Category())
	}
}

// corruptPackIdx overwrites a byte near the end of a pack .idx file (inside
// the trailing idx checksum), so go-git's idx decoder returns
// "malformed IDX file: checksum mismatch: …" or surfaces it as
// "object not found" (when the lookup table is unreadable) the next time
// any object lookup hits that pack. The .idx checksum IS validated on every
// pack access (it's checked when go-git lazily loads the index for the
// pack), so this deterministically breaks the next object read. We corrupt
// the .idx rather than the .pack signature because go-git's per-object
// reader seeks to the object's offset and doesn't re-validate the .pack
// signature — corrupting the .pack on its own is not enough to break
// ordinary reads.
func corruptPackIdx(idxPath string) error {
	if err := os.Chmod(idxPath, 0o600); err != nil {
		return err
	}
	// #nosec G304 -- idxPath is constructed inside the test from a t.TempDir
	// glob, not from untrusted input.
	f, err := os.OpenFile(idxPath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	// Flip a byte 4 bytes before EOF (well inside the trailing SHA-1
	// checksum of the idx file). Xor 0xFF so a coincidental match is
	// essentially impossible.
	pos := max(info.Size()-4, 0)
	buf := make([]byte, 1)
	if _, err := f.ReadAt(buf, pos); err != nil {
		return err
	}
	buf[0] ^= 0xFF
	if _, err := f.WriteAt(buf, pos); err != nil {
		return err
	}
	return nil
}
