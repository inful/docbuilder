package hugo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	ggit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"
	"git.home.luguber.info/inful/docbuilder/internal/version"
)

// A build that must not be skipped copies the content even when nothing
// moved: the corpus is what is sent to ragabast, and a skipped build
// sends nothing.
func TestGenerateFullSite_WithoutSkip_CopiesTheContentWhenNothingMoved(t *testing.T) {
	tests := []struct {
		name          string
		skipIfChanged bool
		wantSkipped   bool
		wantDocuments int
	}{
		{
			name:          "the build may be skipped",
			skipIfChanged: true,
			wantSkipped:   true,
			wantDocuments: 0,
		},
		{
			name:          "the build must not be skipped",
			skipIfChanged: false,
			wantSkipped:   false,
			wantDocuments: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			repoName := "repo1"

			// A local origin repo with one document.
			originDir := filepath.Join(base, "origin")
			originRepo, initErr := ggit.PlainInit(originDir, false)
			if initErr != nil {
				t.Fatalf("init origin repo: %v", initErr)
			}
			if mkErr := os.MkdirAll(filepath.Join(originDir, "docs"), 0o750); mkErr != nil {
				t.Fatalf("mkdir docs: %v", mkErr)
			}
			if writeErr := os.WriteFile(filepath.Join(originDir, "docs", "guide.md"), []byte("# Guide\n"), 0o600); writeErr != nil {
				t.Fatalf("write origin guide: %v", writeErr)
			}
			wt, wtErr := originRepo.Worktree()
			if wtErr != nil {
				t.Fatalf("origin worktree: %v", wtErr)
			}
			if _, addErr := wt.Add("docs/guide.md"); addErr != nil {
				t.Fatalf("origin add: %v", addErr)
			}
			if _, err := wt.Commit("initial", &ggit.CommitOptions{
				Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
			}); err != nil {
				t.Fatalf("origin commit: %v", err)
			}

			cfg := &config.Config{
				Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"},
				Build: config.BuildConfig{
					CloneStrategy:   config.CloneStrategyAuto,
					RenderMode:      config.RenderModeNever,
					SkipIfUnchanged: tt.skipIfChanged,
				},
			}

			// A site of a build that ran before: the build may be
			// skipped, and the skip needs a valid site.
			outDir := filepath.Join(base, "site")
			if err := os.MkdirAll(filepath.Join(outDir, "public"), 0o750); err != nil {
				t.Fatalf("mkdir public: %v", err)
			}
			if err := os.WriteFile(filepath.Join(outDir, "public", "index.html"), []byte("index"), 0o600); err != nil {
				t.Fatalf("write index: %v", err)
			}
			if err := os.MkdirAll(filepath.Join(outDir, "content"), 0o750); err != nil {
				t.Fatalf("mkdir content: %v", err)
			}
			if err := os.WriteFile(filepath.Join(outDir, "content", "_index.md"), []byte("# Content\n"), 0o600); err != nil {
				t.Fatalf("write content md: %v", err)
			}
			if err := os.MkdirAll(filepath.Join(outDir, "content", repoName), 0o750); err != nil {
				t.Fatalf("mkdir content repo dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(outDir, "content", repoName, "_index.md"), []byte("# Repo Section\n"), 0o600); err != nil {
				t.Fatalf("write repo section md: %v", err)
			}
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

			// A workspace with the clone of the origin: the clone stage
			// sees that nothing moved.
			workspaceDir := filepath.Join(base, "ws")
			clonePath := filepath.Join(workspaceDir, repoName)
			if mkErr := os.MkdirAll(workspaceDir, 0o750); mkErr != nil {
				t.Fatalf("mkdir workspace: %v", mkErr)
			}
			if _, cloneErr := ggit.PlainClone(clonePath, false, &ggit.CloneOptions{URL: originDir}); cloneErr != nil {
				t.Fatalf("pre-clone into workspace: %v", cloneErr)
			}

			var (
				mu        sync.Mutex
				documents []string
			)
			gen := NewGenerator(cfg, outDir).
				WithRenderer(&stages.NoopRenderer{}).
				WithDocumentReady(func(_ []byte, path string) {
					mu.Lock()
					documents = append(documents, path)
					mu.Unlock()
				})

			report, err := gen.GenerateFullSite(context.Background(), []config.Repository{{
				Name:   repoName,
				URL:    originDir,
				Branch: "master",
				Paths:  []string{"docs"},
			}}, workspaceDir)
			if err != nil {
				t.Fatalf("GenerateFullSite error: %v", err)
			}

			mu.Lock()
			got := len(documents)
			mu.Unlock()

			if tt.wantSkipped {
				if report.SkipReason != "no_changes" {
					t.Fatalf("the build should have been skipped, skip reason %q", report.SkipReason)
				}
			} else if report.SkipReason != "" {
				t.Fatalf("the build should not have been skipped, skip reason %q", report.SkipReason)
			}
			if got != tt.wantDocuments {
				t.Fatalf("the build emitted %d documents, want %d", got, tt.wantDocuments)
			}
		})
	}
}
