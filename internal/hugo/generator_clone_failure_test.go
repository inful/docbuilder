package hugo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	ggit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"
)

// newLocalOrigin creates a git repository with one markdown file and
// returns its path.
func newLocalOrigin(t *testing.T, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	repo, initErr := ggit.PlainInit(dir, false)
	if initErr != nil {
		t.Fatalf("init the origin repo: %v", initErr)
	}
	if mkdirErr := os.MkdirAll(filepath.Join(dir, "docs"), 0o750); mkdirErr != nil {
		t.Fatalf("make the docs dir: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(filepath.Join(dir, "docs", "README.md"), []byte("# Origin\n"), 0o600); writeErr != nil {
		t.Fatalf("write the readme: %v", writeErr)
	}
	wt, wtErr := repo.Worktree()
	if wtErr != nil {
		t.Fatalf("worktree: %v", wtErr)
	}
	if _, err := wt.Add("docs/README.md"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("initial", &ggit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return dir
}

// Not one repository can be cloned: the build must fail and the site
// must not be published. A site built without any of the repositories
// would take the documentation of the portal offline.
func TestGenerateFullSiteDoesNotPublishWhenEveryCloneFails(t *testing.T) {
	base := t.TempDir()
	outDir := filepath.Join(base, "site")

	cfg := &config.Config{
		Hugo:  config.HugoConfig{Title: "Test", BaseURL: "/"},
		Build: config.BuildConfig{RenderMode: config.RenderModeNever},
	}
	repos := []config.Repository{{Name: "repoA", URL: filepath.Join(base, "does-not-exist")}}

	gen := NewGenerator(cfg, outDir).WithRenderer(&stages.NoopRenderer{})
	report, err := gen.GenerateFullSite(t.Context(), repos, filepath.Join(base, "ws"))
	if err == nil {
		t.Fatalf("the build did not fail: %+v", report)
	}

	if _, statErr := os.Stat(filepath.Join(outDir, "public")); !os.IsNotExist(statErr) {
		t.Fatalf("a site was published anyway: %v", statErr)
	}
}

// One repository of two cannot be cloned: the site of the last build
// still has the pages of that repository, so it stays published and the
// build fails.
func TestGenerateFullSiteKeepsTheOldSiteWhenACloneFails(t *testing.T) {
	base := t.TempDir()
	outDir := filepath.Join(base, "site")
	origin := newLocalOrigin(t, base, "origin")

	// The site of the last build.
	if err := os.MkdirAll(filepath.Join(outDir, "public"), 0o750); err != nil {
		t.Fatalf("make the public dir: %v", err)
	}
	sentinel := []byte("the site of the last build")
	if err := os.WriteFile(filepath.Join(outDir, "public", "index.html"), sentinel, 0o600); err != nil {
		t.Fatalf("write the sentinel: %v", err)
	}

	cfg := &config.Config{
		Hugo:  config.HugoConfig{Title: "Test", BaseURL: "/"},
		Build: config.BuildConfig{RenderMode: config.RenderModeNever},
	}
	repos := []config.Repository{
		{Name: "repoA", URL: origin, Paths: []string{"docs"}},
		{Name: "repoB", URL: filepath.Join(base, "does-not-exist")},
	}

	gen := NewGenerator(cfg, outDir).WithRenderer(&stages.NoopRenderer{})
	report, err := gen.GenerateFullSite(t.Context(), repos, filepath.Join(base, "ws"))
	if err == nil {
		t.Fatalf("the build did not fail: %+v", report)
	}

	// The report says that the build failed: the site is missing a
	// repository, and the report in the site says so.
	if report.Outcome != models.OutcomeFailed {
		t.Fatalf("the outcome of the build is %q, want %q", report.Outcome, models.OutcomeFailed)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "public", "index.html")) // #nosec G304 -- the path is the test's own
	if err != nil {
		t.Fatalf("read the site: %v", err)
	}
	if !bytes.Equal(got, sentinel) {
		t.Fatalf("the site of the last build was replaced: %s", got)
	}
}
