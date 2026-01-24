package hugo

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	ggit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"
	"git.home.luguber.info/inful/docbuilder/internal/version"
)

func TestGenerateFullSite_EarlySkip_DoesNotFinalizeStaging(t *testing.T) {
	base := t.TempDir()
	repoName := "repo1"

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

	// Prepare config early so we can seed a realistic build-report.json.
	cfg := &config.Config{
		Hugo:   config.HugoConfig{Title: "Test", BaseURL: "/"},
		Build:  config.BuildConfig{CloneStrategy: config.CloneStrategyAuto, RenderMode: config.RenderModeNever},
		Output: config.OutputConfig{},
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
	// existingSiteValidForSkip() requires at least one markdown file besides the root content/_index.md.
	if err := os.MkdirAll(filepath.Join(outDir, "content", repoName), 0o750); err != nil {
		t.Fatalf("mkdir content repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "content", repoName, "_index.md"), []byte("# Repo Section\n"), 0o600); err != nil {
		t.Fatalf("write repo section md: %v", err)
	}
	gen := NewGenerator(cfg, outDir).WithRenderer(&stages.NoopRenderer{})
	prev := map[string]any{
		"schema_version":        1,
		"repositories":          1,
		"files":                 2,
		"start":                 time.Now().UTC(),
		"end":                   time.Now().UTC(),
		"errors":                []string{},
		"warnings":              []string{},
		"stage_durations":       map[string]any{},
		"stage_error_kinds":     map[string]any{},
		"cloned_repositories":   1,
		"failed_repositories":   0,
		"skipped_repositories":  0,
		"rendered_pages":        0,
		"stage_counts":          map[string]any{},
		"outcome":               "success",
		"static_rendered":       true,
		"retries":               0,
		"retries_exhausted":     false,
		"issues":                []any{},
		"config_hash":           cfg.Snapshot(),
		"pipeline_version":      1,
		"effective_render_mode": string(cfg.Build.RenderMode),
		"docbuilder_version":    version.Version,
		"hugo_version":          "",
	}
	jb, marshalErr := json.Marshal(prev)
	if marshalErr != nil {
		t.Fatalf("marshal build report: %v", marshalErr)
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, "build-report.json"), jb, 0o600); writeErr != nil {
		t.Fatalf("write build report: %v", writeErr)
	}

	// Prepare a workspace clone so clone stage can detect preHead and compute "unchanged".
	workspaceDir := filepath.Join(base, "ws")
	clonePath := filepath.Join(workspaceDir, repoName)
	if mkErr := os.MkdirAll(workspaceDir, 0o750); mkErr != nil {
		t.Fatalf("mkdir workspace: %v", mkErr)
	}
	if _, cloneErr := ggit.PlainClone(clonePath, false, &ggit.CloneOptions{URL: originDir}); cloneErr != nil {
		t.Fatalf("pre-clone into workspace: %v", cloneErr)
	}

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

func TestGenerateFullSite_EarlySkip_RequiresPublicIndexHTML(t *testing.T) {
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

	// Prepare workspace clone so clone stage can detect preHead and compute "unchanged".
	workspaceDir := filepath.Join(base, "ws")
	repoName := "repo1"
	clonePath := filepath.Join(workspaceDir, repoName)
	if err := os.MkdirAll(workspaceDir, 0o750); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if _, err := ggit.PlainClone(clonePath, false, &ggit.CloneOptions{URL: originDir}); err != nil {
		t.Fatalf("pre-clone into workspace: %v", err)
	}

	// Seed an existing output directory that *looks* non-empty but is missing public/index.html.
	outDir := filepath.Join(base, "site")
	if err := os.MkdirAll(filepath.Join(outDir, "public", "assets"), 0o750); err != nil {
		t.Fatalf("mkdir public assets: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "content", "_index.md"), []byte("# Content\n"), 0o600); err != nil {
		t.Fatalf("write content md: %v", err)
	}

	cfg := &config.Config{
		Hugo:   config.HugoConfig{Title: "Test", BaseURL: "/"},
		Build:  config.BuildConfig{CloneStrategy: config.CloneStrategyAuto, RenderMode: config.RenderModeNever},
		Output: config.OutputConfig{},
	}
	prev := map[string]any{
		"schema_version":        1,
		"repositories":          1,
		"files":                 1,
		"start":                 time.Now().UTC(),
		"end":                   time.Now().UTC(),
		"errors":                []string{},
		"warnings":              []string{},
		"stage_durations":       map[string]any{},
		"stage_error_kinds":     map[string]any{},
		"cloned_repositories":   1,
		"failed_repositories":   0,
		"skipped_repositories":  0,
		"rendered_pages":        0,
		"stage_counts":          map[string]any{},
		"outcome":               "success",
		"static_rendered":       true,
		"retries":               0,
		"retries_exhausted":     false,
		"issues":                []any{},
		"config_hash":           cfg.Snapshot(),
		"pipeline_version":      1,
		"effective_render_mode": string(cfg.Build.RenderMode),
		"docbuilder_version":    version.Version,
		"hugo_version":          "",
	}
	jb, marshalErr := json.Marshal(prev)
	if marshalErr != nil {
		t.Fatalf("marshal build report: %v", marshalErr)
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, "build-report.json"), jb, 0o600); writeErr != nil {
		t.Fatalf("write build report: %v", writeErr)
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
	if report.SkipReason == "no_changes" {
		t.Fatalf("expected early skip to be rejected when public/index.html is missing")
	}
}

func TestExistingSiteValidForSkip_RejectsZeroFilesReport(t *testing.T) {
	gen := setupExistingSiteValidForSkipProbe(t, 0)
	if gen.ExistingSiteValidForSkip() {
		t.Fatalf("expected ExistingSiteValidForSkip()=false when previous report files==0")
	}
}

func TestExistingSiteValidForSkip_RejectsOnlyRootIndexContent(t *testing.T) {
	gen := setupExistingSiteValidForSkipProbe(t, 1)
	if gen.ExistingSiteValidForSkip() {
		t.Fatalf("expected ExistingSiteValidForSkip()=false when content only contains root _index.md")
	}
}

func setupExistingSiteValidForSkipProbe(t *testing.T, reportFiles int) *Generator {
	t.Helper()

	base := t.TempDir()
	outDir := filepath.Join(base, "site")

	if err := os.MkdirAll(filepath.Join(outDir, "public"), 0o750); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "public", "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "content", "_index.md"), []byte("# Root\n"), 0o600); err != nil {
		t.Fatalf("write content: %v", err)
	}

	cfg := &config.Config{
		Hugo:   config.HugoConfig{Title: "Test", BaseURL: "/"},
		Build:  config.BuildConfig{CloneStrategy: config.CloneStrategyAuto, RenderMode: config.RenderModeNever},
		Output: config.OutputConfig{},
	}
	gen := NewGenerator(cfg, outDir).WithRenderer(&stages.NoopRenderer{})

	prev := map[string]any{
		"schema_version":        1,
		"repositories":          1,
		"files":                 reportFiles,
		"start":                 time.Now().UTC(),
		"end":                   time.Now().UTC(),
		"errors":                []string{},
		"warnings":              []string{},
		"stage_durations":       map[string]any{},
		"stage_error_kinds":     map[string]any{},
		"cloned_repositories":   1,
		"failed_repositories":   0,
		"skipped_repositories":  0,
		"rendered_pages":        0,
		"stage_counts":          map[string]any{},
		"outcome":               "success",
		"static_rendered":       true,
		"retries":               0,
		"retries_exhausted":     false,
		"issues":                []any{},
		"config_hash":           cfg.Snapshot(),
		"pipeline_version":      1,
		"effective_render_mode": string(cfg.Build.RenderMode),
		"docbuilder_version":    version.Version,
		"hugo_version":          "",
	}
	jb, err := json.Marshal(prev)
	if err != nil {
		t.Fatalf("marshal build report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "build-report.json"), jb, 0o600); err != nil {
		t.Fatalf("write build report: %v", err)
	}

	return gen
}
