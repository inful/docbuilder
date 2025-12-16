package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// fakeSkipState is a lightweight in-memory implementation of SkipStateAccess for testing.
type fakeSkipState struct {
	repoLastCommit     map[string]string
	repoDocHash        map[string]string
	lastConfigHash     string
	lastReportChecksum string
	lastGlobalDocFiles string
}

func newFakeSkipState() *fakeSkipState {
	return &fakeSkipState{
		repoLastCommit: map[string]string{},
		repoDocHash:    map[string]string{},
	}
}

func (f *fakeSkipState) GetRepoLastCommit(u string) string   { return f.repoLastCommit[u] }
func (f *fakeSkipState) GetLastConfigHash() string           { return f.lastConfigHash }
func (f *fakeSkipState) GetLastReportChecksum() string       { return f.lastReportChecksum }
func (f *fakeSkipState) SetLastReportChecksum(s string)      { f.lastReportChecksum = s }
func (f *fakeSkipState) GetRepoDocFilesHash(u string) string { return f.repoDocHash[u] }
func (f *fakeSkipState) GetLastGlobalDocFilesHash() string   { return f.lastGlobalDocFiles }
func (f *fakeSkipState) SetLastGlobalDocFilesHash(s string)  { f.lastGlobalDocFiles = s }

// minimal generator wrapper for config hash computation only (embed real generator)
func newTestGenerator(_ *testing.T, cfg *cfg.Config, outDir string) *hugo.Generator {
	return hugo.NewGenerator(cfg, outDir)
}

// writePrevReport helper writes a build-report.json with the provided fields and updates checksum state.
func writePrevReport(t *testing.T, outDir string, repos, files, rendered int, docHash string, st *fakeSkipState) {
	// Reference unused parameter to satisfy unparam
	// nolint:unparam // Test helper accepts fixed values in call sites by design.
	_ = repos
	prev := struct {
		Repositories      int    `json:"repositories"`
		Files             int    `json:"files"`
		RenderedPages     int    `json:"rendered_pages"`
		DocFilesHash      string `json:"doc_files_hash"`
		DocBuilderVersion string `json:"doc_builder_version"`
		HugoVersion       string `json:"hugo_version"`
	}{repos, files, rendered, docHash, "2.1.0-dev", ""} // Match current version
	b, err := json.Marshal(prev)
	if err != nil {
		t.Fatalf("marshal prev: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "build-report.json"), b, 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	sum := sha256.Sum256(b)
	st.lastReportChecksum = hex.EncodeToString(sum[:])
}

func makeBaseConfig(out string) *cfg.Config {
	return &cfg.Config{Output: cfg.OutputConfig{Directory: out}, Hugo: cfg.HugoConfig{}}
}

// TestSkipEvaluator_SkipHappyPath validates that all guards satisfied produces a skip report.
func TestSkipEvaluator_SkipHappyPath(t *testing.T) {
	out := t.TempDir()
	// create minimal public and content structures
	pubDir := filepath.Join(out, "public")
	if err := os.MkdirAll(pubDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// create a non-empty artifact so public dir is considered valid
	if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	contentDir := filepath.Join(out, "content")
	if err := os.MkdirAll(contentDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "doc.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	st := newFakeSkipState()
	repo := cfg.Repository{Name: "r1", URL: "https://example.com/r1.git", Branch: "main"}
	conf := makeBaseConfig(out)
	conf.Repositories = []cfg.Repository{repo}
	gen := newTestGenerator(t, conf, out)
	st.lastConfigHash = gen.ComputeConfigHashForPersistence()
	st.repoLastCommit[repo.URL] = "deadbeef"
	st.repoDocHash[repo.URL] = "abc123"
	writePrevReport(t, out, 1, 2, 2, "abc123", st)
	st.lastGlobalDocFiles = "abc123"
	rep, ok := NewSkipEvaluator(out, st, gen).Evaluate([]cfg.Repository{repo})
	if !ok {
		t.Fatalf("expected skip")
	}
	if rep.SkipReason != "no_changes" || rep.Repositories != 1 || rep.Files != 2 {
		t.Fatalf("unexpected report values: %+v", rep)
	}
}

// TestSkipEvaluator_ConfigHashChange forces rebuild when config hash mismatches.
func TestSkipEvaluator_ConfigHashChange(t *testing.T) {
	out := t.TempDir()
	pubDir := filepath.Join(out, "public")
	if err := os.MkdirAll(pubDir, 0o750); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(out, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "content", "doc.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatalf("write doc.md: %v", err)
	}
	st := newFakeSkipState()
	repo := cfg.Repository{Name: "r1", URL: "u", Branch: "main"}
	conf1 := makeBaseConfig(out)
	conf1.Hugo.Title = "A"
	conf1.Repositories = []cfg.Repository{repo}
	gen1 := newTestGenerator(t, conf1, out)
	writePrevReport(t, out, 1, 1, 1, "h1", st)
	// store old hash then mutate config to change hash
	st.lastConfigHash = gen1.ComputeConfigHashForPersistence()
	conf1.Hugo.Title = "B" // forces a different hash for new generator
	gen2 := newTestGenerator(t, conf1, out)
	st.repoLastCommit[repo.URL] = "c1"
	st.repoDocHash[repo.URL] = "h1"
	st.lastGlobalDocFiles = "h1"
	if rep, ok := NewSkipEvaluator(out, st, gen2).Evaluate([]cfg.Repository{repo}); ok || rep != nil {
		t.Fatalf("expected rebuild due to config hash change")
	}
}

// TestSkipEvaluator_PublicDirMissing triggers rebuild when public/ missing.
func TestSkipEvaluator_PublicDirMissing(t *testing.T) {
	out := t.TempDir()
	if err := os.MkdirAll(filepath.Join(out, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "content", "doc.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatalf("write doc.md: %v", err)
	}
	st := newFakeSkipState()
	repo := cfg.Repository{Name: "r1", URL: "u", Branch: "main"}
	conf1 := makeBaseConfig(out)
	conf1.Repositories = []cfg.Repository{repo}
	gen := newTestGenerator(t, conf1, out)
	st.lastConfigHash = gen.ComputeConfigHashForPersistence()
	st.repoLastCommit[repo.URL] = "c1"
	st.repoDocHash[repo.URL] = "h1"
	writePrevReport(t, out, 1, 1, 1, "h1", st)
	st.lastGlobalDocFiles = "h1"
	if rep, ok := NewSkipEvaluator(out, st, gen).Evaluate([]cfg.Repository{repo}); ok || rep != nil {
		t.Fatalf("expected rebuild (no public dir)")
	}
}

// TestSkipEvaluator_PerRepoHashMismatch ensures mismatch forces rebuild.
func TestSkipEvaluator_PerRepoHashMismatch(t *testing.T) {
	out := t.TempDir()
	pubDir := filepath.Join(out, "public")
	if err := os.MkdirAll(pubDir, 0o750); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(out, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "content", "doc.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatalf("write doc.md: %v", err)
	}
	st := newFakeSkipState()
	repo := cfg.Repository{Name: "r1", URL: "u", Branch: "main"}
	conf1 := makeBaseConfig(out)
	conf1.Repositories = []cfg.Repository{repo}
	gen := newTestGenerator(t, conf1, out)
	st.lastConfigHash = gen.ComputeConfigHashForPersistence()
	st.repoLastCommit[repo.URL] = "c1"
	st.repoDocHash[repo.URL] = "other" // mismatch with report
	writePrevReport(t, out, 1, 1, 1, "match", st)
	st.lastGlobalDocFiles = "match"
	if rep, ok := NewSkipEvaluator(out, st, gen).Evaluate([]cfg.Repository{repo}); ok || rep != nil {
		t.Fatalf("expected rebuild (per-repo hash mismatch)")
	}
}

// TestSkipEvaluator_GlobalHashMismatch ensures stored global vs report mismatch forces rebuild.
func TestSkipEvaluator_GlobalHashMismatch(t *testing.T) {
	out := t.TempDir()
	pubDir := filepath.Join(out, "public")
	if err := os.MkdirAll(pubDir, 0o750); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(out, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "content", "doc.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatalf("write doc.md: %v", err)
	}
	st := newFakeSkipState()
	repo := cfg.Repository{Name: "r1", URL: "u", Branch: "main"}
	conf1 := makeBaseConfig(out)
	conf1.Repositories = []cfg.Repository{repo}
	gen := newTestGenerator(t, conf1, out)
	st.lastConfigHash = gen.ComputeConfigHashForPersistence()
	st.repoLastCommit[repo.URL] = "c1"
	st.repoDocHash[repo.URL] = "H" // matches report but global differs
	writePrevReport(t, out, 1, 1, 1, "H", st)
	st.lastGlobalDocFiles = "DIFF"
	if rep, ok := NewSkipEvaluator(out, st, gen).Evaluate([]cfg.Repository{repo}); ok || rep != nil {
		t.Fatalf("expected rebuild (global hash mismatch)")
	}
}

// TestSkipEvaluator_MissingCommit forces rebuild when commit metadata absent.
func TestSkipEvaluator_MissingCommit(t *testing.T) {
	out := t.TempDir()
	pubDir := filepath.Join(out, "public")
	if err := os.MkdirAll(pubDir, 0o750); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(out, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "content", "doc.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatalf("write doc.md: %v", err)
	}
	st := newFakeSkipState()
	repo := cfg.Repository{Name: "r1", URL: "u", Branch: "main"}
	conf1 := makeBaseConfig(out)
	conf1.Repositories = []cfg.Repository{repo}
	gen := newTestGenerator(t, conf1, out)
	st.lastConfigHash = gen.ComputeConfigHashForPersistence()
	writePrevReport(t, out, 1, 1, 1, "H", st)
	st.repoDocHash[repo.URL] = "H"
	st.lastGlobalDocFiles = "H"
	if rep, ok := NewSkipEvaluator(out, st, gen).Evaluate([]cfg.Repository{repo}); ok || rep != nil {
		t.Fatalf("expected rebuild (missing commit)")
	}
}

// Ensure future changes don't remove time usage accidentally (sanity for timestamps in report persistence when skipping).
func TestSkipEvaluator_SetsTimestampsOnSkip(t *testing.T) {
	out := t.TempDir()
	pubDir := filepath.Join(out, "public")
	if err := os.MkdirAll(pubDir, 0o750); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(out, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "content", "doc.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatalf("write doc.md: %v", err)
	}
	st := newFakeSkipState()
	repo := cfg.Repository{Name: "r1", URL: "u", Branch: "main"}
	cfg1 := makeBaseConfig(out)
	cfg1.Repositories = []cfg.Repository{repo}
	gen := newTestGenerator(t, cfg1, out)
	st.lastConfigHash = gen.ComputeConfigHashForPersistence()
	st.repoLastCommit[repo.URL] = "c1"
	st.repoDocHash[repo.URL] = "X"
	writePrevReport(t, out, 1, 5, 5, "X", st)
	st.lastGlobalDocFiles = "X"
	rep, ok := NewSkipEvaluator(out, st, gen).Evaluate([]cfg.Repository{repo})
	if !ok || rep == nil {
		t.Fatalf("expected skip")
	}
	if rep.Start.IsZero() || rep.End.IsZero() || rep.End.Before(rep.Start) {
		t.Fatalf("timestamps not set correctly: %#v", rep)
	}
	// Ensure checksum updated
	// #nosec G304 - test file path
	raw, err := os.ReadFile(filepath.Join(out, "build-report.json"))
	if err != nil {
		t.Fatalf("read persisted report: %v", err)
	}
	sum := sha256.Sum256(raw)
	if st.lastReportChecksum != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum not updated on skip")
	}
}

// TestSkipEvaluator_VersionMismatch ensures version changes force rebuild.
func TestSkipEvaluator_VersionMismatch(t *testing.T) {
	out := t.TempDir()
	pubDir := filepath.Join(out, "public")
	if err := os.MkdirAll(pubDir, 0o750); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(out, "content"), 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "content", "doc.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatalf("write doc.md: %v", err)
	}
	st := newFakeSkipState()
	repo := cfg.Repository{Name: "r1", URL: "u", Branch: "main"}
	conf1 := makeBaseConfig(out)
	conf1.Repositories = []cfg.Repository{repo}
	gen := newTestGenerator(t, conf1, out)
	st.lastConfigHash = gen.ComputeConfigHashForPersistence()
	st.repoLastCommit[repo.URL] = "c1"
	st.repoDocHash[repo.URL] = "h1"

	// Write previous report with different DocBuilder version
	writePrevReportWithVersions(t, out, 1, 1, 1, "h1", st, "1.0.0", "")
	st.lastGlobalDocFiles = "h1"

	// Evaluate should force rebuild due to version mismatch
	if rep, ok := NewSkipEvaluator(out, st, gen).Evaluate([]cfg.Repository{repo}); ok || rep != nil {
		t.Fatalf("expected rebuild due to version mismatch (docbuilder)")
	}
}

// writePrevReportWithVersions is like writePrevReport but includes version fields.
func writePrevReportWithVersions(t *testing.T, outDir string, repos, files, rendered int, docHash string, st *fakeSkipState, dbVersion, hugoVersion string) {
	t.Helper()
	report := map[string]any{
		"schema_version":        1,
		"repositories":          repos,
		"files":                 files,
		"rendered_pages":        rendered,
		"doc_files_hash":        docHash,
		"doc_builder_version":   dbVersion,
		"hugo_version":          hugoVersion,
		"outcome":               "success",
		"start":                 time.Now().Format(time.RFC3339),
		"end":                   time.Now().Format(time.RFC3339),
		"stage_durations":       map[string]string{},
		"stage_error_kinds":     map[string]string{},
		"stage_counts":          map[string]any{},
		"index_templates":       map[string]any{},
		"delta_repo_reasons":    map[string]string{},
		"errors":                []string{},
		"warnings":              []string{},
		"issues":                []any{},
		"cloned_repositories":   0,
		"failed_repositories":   0,
		"skipped_repositories":  0,
		"static_rendered":       false,
		"retries":               0,
		"retries_exhausted":     false,
		"skip_reason":           "",
		"clone_stage_skipped":   false,
		"delta_decision":        "",
		"delta_changed_repos":   []string{},
		"config_hash":           "",
		"pipeline_version":      0,
		"effective_render_mode": "",
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	path := filepath.Join(outDir, "build-report.json")
	// #nosec G306 - test file permissions
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	sum := sha256.Sum256(data)
	st.lastReportChecksum = hex.EncodeToString(sum[:])
}
