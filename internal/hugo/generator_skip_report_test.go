package hugo

import (
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

// A build that was skipped writes a report without files: it did not
// discover any. The next build reads that report to decide whether it
// may skip, and a report without files is read as "the site is empty" -
// so every second build was a full build of the whole portal.
//
// The report of a skipped build keeps the numbers of the last build that
// ran.
func TestSkippedBuildKeepsTheNumbersOfTheLastBuild(t *testing.T) {
	base := t.TempDir()
	repoName := "repo1"

	originDir := filepath.Join(base, "origin")
	originRepo, initErr := ggit.PlainInit(originDir, false)
	if initErr != nil {
		t.Fatalf("init origin repo: %v", initErr)
	}
	if mkErr := os.MkdirAll(filepath.Join(originDir, "docs"), 0o750); mkErr != nil {
		t.Fatalf("mkdir docs: %v", mkErr)
	}
	if writeErr := os.WriteFile(filepath.Join(originDir, "docs", "README.md"), []byte("# Origin\n"), 0o600); writeErr != nil {
		t.Fatalf("write origin readme: %v", writeErr)
	}
	wt, wtErr := originRepo.Worktree()
	if wtErr != nil {
		t.Fatalf("origin worktree: %v", wtErr)
	}
	if _, addErr := wt.Add("docs/README.md"); addErr != nil {
		t.Fatalf("origin add: %v", addErr)
	}
	if _, commitErr := wt.Commit("initial", &ggit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	}); commitErr != nil {
		t.Fatalf("origin commit: %v", commitErr)
	}

	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"},
		Build: config.BuildConfig{
			CloneStrategy:   config.CloneStrategyAuto,
			RenderMode:      config.RenderModeNever,
			SkipIfUnchanged: true,
		},
	}

	outDir := filepath.Join(base, "site")
	if mkErr := os.MkdirAll(filepath.Join(outDir, "public"), 0o750); mkErr != nil {
		t.Fatalf("mkdir public: %v", mkErr)
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, "public", "index.html"), []byte("sentinel"), 0o600); writeErr != nil {
		t.Fatalf("write sentinel: %v", writeErr)
	}
	if mkErr := os.MkdirAll(filepath.Join(outDir, "content", repoName), 0o750); mkErr != nil {
		t.Fatalf("mkdir content repo dir: %v", mkErr)
	}
	if writeErr := os.WriteFile(filepath.Join(outDir, "content", repoName, "_index.md"), []byte("# Repo\n"), 0o600); writeErr != nil {
		t.Fatalf("write repo section md: %v", writeErr)
	}

	prev := map[string]any{
		"schema_version":        1,
		"repositories":          1,
		"files":                 7,
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

	workspaceDir := filepath.Join(base, "ws")
	clonePath := filepath.Join(workspaceDir, repoName)
	if mkErr := os.MkdirAll(workspaceDir, 0o750); mkErr != nil {
		t.Fatalf("mkdir workspace: %v", mkErr)
	}
	if _, cloneErr := ggit.PlainClone(clonePath, false, &ggit.CloneOptions{URL: originDir}); cloneErr != nil {
		t.Fatalf("pre-clone into workspace: %v", cloneErr)
	}

	gen := NewGenerator(cfg, outDir).WithRenderer(&stages.NoopRenderer{})
	report, genErr := gen.GenerateFullSite(t.Context(), []config.Repository{{
		Name:   repoName,
		URL:    originDir,
		Branch: "master",
		Paths:  []string{"docs"},
	}}, workspaceDir)
	if genErr != nil {
		t.Fatalf("GenerateFullSite error: %v", genErr)
	}
	if report.SkipReason != "no_changes" {
		t.Fatalf("expected the build to be skipped, got %q", report.SkipReason)
	}

	// The report that was written must hold the numbers of the site, so
	// the next build may skip as well.
	// #nosec G304 -- controlled test path rooted in t.TempDir()
	written, readErr := os.ReadFile(filepath.Join(outDir, "build-report.json"))
	if readErr != nil {
		t.Fatalf("read the report: %v", readErr)
	}
	var got map[string]any
	if unmarshalErr := json.Unmarshal(written, &got); unmarshalErr != nil {
		t.Fatalf("read the report: %v", unmarshalErr)
	}
	if got["files"] != float64(7) {
		t.Fatalf("the report holds %v files, want 7", got["files"])
	}
	if got["repositories"] != float64(1) {
		t.Fatalf("the report holds %v repositories, want 1", got["repositories"])
	}
}
