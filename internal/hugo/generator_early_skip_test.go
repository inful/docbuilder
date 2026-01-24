package hugo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	ggit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"
)

func TestGenerateFullSite_EarlySkip_DoesNotFinalizeStaging(t *testing.T) {
	base := t.TempDir()

	// Create a local origin repo with at least one commit.
	originDir := filepath.Join(base, "origin")
	originRepo, initErr := ggit.PlainInit(originDir, false)
	if initErr != nil {
		t.Fatalf("init origin repo: %v", initErr)
	}
	if err := os.MkdirAll(filepath.Join(originDir, "docs"), 0o750); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(originDir, "docs", "README.md"), []byte("# Origin\n"), 0o600); err != nil {
		t.Fatalf("write origin readme: %v", err)
	}
	wt, wtErr := originRepo.Worktree()
	if wtErr != nil {
		t.Fatalf("origin worktree: %v", wtErr)
	}
	if _, err := wt.Add("docs/README.md"); err != nil {
		t.Fatalf("origin add: %v", err)
	}
	_, commitErr := wt.Commit("initial", &ggit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	if commitErr != nil {
		t.Fatalf("origin commit: %v", commitErr)
	}

	// Seed an existing *valid* output site that must not be replaced.
	outDir := filepath.Join(base, "site")
	if err := os.MkdirAll(filepath.Join(outDir, "public"), 0o750); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	sentinel := []byte("sentinel")
	if err := os.WriteFile(filepath.Join(outDir, "public", "index.html"), sentinel, 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "content", "_index.md"), []byte("# Content\n"), 0o600); err != nil {
		t.Fatalf("write content md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "build-report.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write build report: %v", err)
	}

	// Prepare a workspace clone so clone stage can detect preHead and compute "unchanged".
	workspaceDir := filepath.Join(base, "ws")
	repoName := "repo1"
	clonePath := filepath.Join(workspaceDir, repoName)
	if err := os.MkdirAll(workspaceDir, 0o750); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if _, err := ggit.PlainClone(clonePath, false, &ggit.CloneOptions{URL: originDir}); err != nil {
		t.Fatalf("pre-clone into workspace: %v", err)
	}

	cfg := &config.Config{
		Hugo:   config.HugoConfig{Title: "Test", BaseURL: "/"},
		Build:  config.BuildConfig{CloneStrategy: config.CloneStrategyAuto, RenderMode: config.RenderModeNever},
		Output: config.OutputConfig{},
	}
	gen := NewGenerator(cfg, outDir).WithRenderer(&stages.NoopRenderer{})

	report, genErr := gen.GenerateFullSite(context.Background(), []config.Repository{{
		Name:   repoName,
		URL:    originDir,
		Branch: "master",
		Paths:  []string{"docs"},
	}}, workspaceDir)
	if genErr != nil {
		t.Fatalf("GenerateFullSite error: %v", genErr)
	}
	if report.SkipReason != "no_changes" {
		t.Fatalf("expected SkipReason=no_changes, got %q", report.SkipReason)
	}

	// Regression check: early skip must not finalize staging (i.e., must not rename output dir away).
	if _, err := os.Stat(outDir + ".prev"); err == nil {
		t.Fatalf("unexpected backup dir created: %s.prev", outDir)
	}
	if _, err := os.Stat(outDir + "_stage"); !os.IsNotExist(err) {
		t.Fatalf("expected staging dir cleaned up, stat err=%v", err)
	}
	// #nosec G304 -- controlled test path rooted in t.TempDir()
	got, err := os.ReadFile(filepath.Join(outDir, "public", "index.html"))
	if err != nil {
		t.Fatalf("read sentinel after skip: %v", err)
	}
	if !bytes.Equal(got, sentinel) {
		t.Fatalf("sentinel changed; expected %q got %q", string(sentinel), string(got))
	}
}
