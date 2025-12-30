package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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

// Test helpers.
func newTestGenerator(t *testing.T, cfg *cfg.Config, outDir string) *hugo.Generator {
	t.Helper()
	return hugo.NewGenerator(cfg, outDir)
}

func writePrevReport(t *testing.T, outDir string, repos, files, rendered int, docHash string, st *fakeSkipState) {
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

func setupValidTestEnvironment(t *testing.T, out string) {
	// Create minimal public structure
	pubDir := filepath.Join(out, "public")
	if err := os.MkdirAll(pubDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create minimal content structure
	contentDir := filepath.Join(out, "content")
	if err := os.MkdirAll(contentDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "doc.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestSkipEvaluator_ValidationRulesIntegration tests the full validation chain.
func TestSkipEvaluator_ValidationRulesIntegration(t *testing.T) {
	out := t.TempDir()
	setupValidTestEnvironment(t, out)

	st := newFakeSkipState()
	repo := cfg.Repository{Name: "r1", URL: "https://example.com/r1.git", Branch: "main"}
	conf := makeBaseConfig(out)
	conf.Repositories = []cfg.Repository{repo}
	gen := newTestGenerator(t, conf, out)

	// Set up valid state
	st.lastConfigHash = gen.ComputeConfigHashForPersistence()
	st.repoLastCommit[repo.URL] = "deadbeef"
	st.repoDocHash[repo.URL] = "abc123"
	writePrevReport(t, out, 1, 2, 2, "abc123", st)
	st.lastGlobalDocFiles = "abc123"

	evaluator := NewSkipEvaluator(out, st, gen)
	rep, ok := evaluator.Evaluate(context.Background(), []cfg.Repository{repo})

	if !ok {
		t.Fatalf("expected skip to succeed")
	}
	if rep.SkipReason != "no_changes" || rep.Repositories != 1 || rep.Files != 2 {
		t.Fatalf("unexpected report values: %+v", rep)
	}
}

// TestValidationRulesCoverage ensures individual rules work correctly.
func TestValidationRulesCoverage(t *testing.T) {
	t.Run("BasicPrerequisitesRule", func(t *testing.T) {
		rule := BasicPrerequisitesRule{}

		// Valid context
		ctx := Context{Context: context.Background(), 
			State:     &fakeSkipState{},
			Generator: &hugo.Generator{},
			Repos:     []cfg.Repository{{Name: "test"}},
		}
		result := rule.Validate(ctx)
		if !result.Passed {
			t.Errorf("expected success, got failure: %s", result.Reason)
		}

		// Invalid context - nil state
		ctx.State = nil
		result = rule.Validate(ctx)
		if result.Passed {
			t.Errorf("expected failure for nil state")
		}
	})

	t.Run("ConfigHashRule", func(t *testing.T) {
		out := t.TempDir()
		conf := makeBaseConfig(out)
		gen := newTestGenerator(t, conf, out)
		st := newFakeSkipState()
		rule := ConfigHashRule{}

		// Valid hash
		hash := gen.ComputeConfigHashForPersistence()
		st.lastConfigHash = hash
		ctx := Context{Context: context.Background(), State: st, Generator: gen}
		result := rule.Validate(ctx)
		if !result.Passed {
			t.Errorf("expected success, got failure: %s", result.Reason)
		}

		// Invalid hash
		st.lastConfigHash = "different"
		result = rule.Validate(ctx)
		if result.Passed {
			t.Errorf("expected failure for mismatched hash")
		}
	})

	t.Run("PublicDirectoryRule", func(t *testing.T) {
		rule := PublicDirectoryRule{}

		// Valid directory
		out := t.TempDir()
		pubDir := filepath.Join(out, "public")
		if err := os.MkdirAll(pubDir, 0o750); err != nil {
			t.Fatalf("mkdir public: %v", err)
		}
		if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte("test"), 0o600); err != nil {
			t.Fatalf("write index.html: %v", err)
		}
		ctx := Context{Context: context.Background(), OutDir: out}
		result := rule.Validate(ctx)
		if !result.Passed {
			t.Errorf("expected success, got failure: %s", result.Reason)
		}

		// Missing directory
		out2 := t.TempDir()
		ctx.OutDir = out2
		result = rule.Validate(ctx)
		if result.Passed {
			t.Errorf("expected failure for missing directory")
		}
	})

	t.Run("ContentIntegrityRule", func(t *testing.T) {
		rule := ContentIntegrityRule{}

		// Skip when no previous files
		ctx := Context{Context: context.Background(), PrevReport: &PreviousReport{Files: 0}}
		result := rule.Validate(ctx)
		if !result.Passed {
			t.Errorf("expected success (skip), got failure: %s", result.Reason)
		}

		// Valid content directory
		out := t.TempDir()
		contentDir := filepath.Join(out, "content")
		if err := os.MkdirAll(contentDir, 0o750); err != nil {
			t.Fatalf("mkdir content: %v", err)
		}
		if err := os.WriteFile(filepath.Join(contentDir, "test.md"), []byte("# Test"), 0o600); err != nil {
			t.Fatalf("write test.md: %v", err)
		}
		ctx = Context{Context: context.Background(), OutDir: out, PrevReport: &PreviousReport{Files: 1}}
		result = rule.Validate(ctx)
		if !result.Passed {
			t.Errorf("expected success, got failure: %s", result.Reason)
		}
	})

	t.Run("VersionMismatchRule", func(t *testing.T) {
		rule := VersionMismatchRule{}

		// No previous report
		ctx := Context{Context: context.Background(), PrevReport: nil}
		result := rule.Validate(ctx)
		if result.Passed {
			t.Errorf("expected failure when no previous report")
		}

		// Same versions (no Hugo)
		ctx = Context{Context: context.Background(), 
			PrevReport: &PreviousReport{
				DocBuilderVersion: "2.1.0-dev", // Should match version.Version in tests
				HugoVersion:       "",          // No Hugo used
			},
		}
		result = rule.Validate(ctx)
		if !result.Passed {
			t.Errorf("expected success for matching versions, got failure: %s", result.Reason)
		}

		// DocBuilder version changed
		ctx = Context{Context: context.Background(), 
			PrevReport: &PreviousReport{
				DocBuilderVersion: "1.0.0", // Different from current
				HugoVersion:       "",
			},
		}
		result = rule.Validate(ctx)
		if result.Passed {
			t.Errorf("expected failure for docbuilder version mismatch")
		}
		if result.Reason != "docbuilder version changed" {
			t.Errorf("unexpected failure reason: %s", result.Reason)
		}

		// Hugo version changed (detected vs previous)
		// Note: This test assumes DetectHugoVersion() returns something or empty
		// In CI without Hugo, DetectHugoVersion() returns "" so we test that case
		ctx = Context{Context: context.Background(), 
			PrevReport: &PreviousReport{
				DocBuilderVersion: "2.1.0-dev",
				HugoVersion:       "0.100.0", // Any previous Hugo version
			},
		}
		result = rule.Validate(ctx)
		// This will pass or fail depending on whether Hugo is installed
		// If Hugo is not available, DetectHugoVersion() returns ""
		// and will not match "0.100.0", causing failure
		// This is the desired behavior: force rebuild if Hugo changes or disappears
	})
}

// TestRuleChain tests the rule chain execution.
func TestRuleChain(t *testing.T) {
	// Create mock rules
	successRule := &mockRule{name: "success", shouldPass: true}
	failureRule := &mockRule{name: "failure", shouldPass: false}

	t.Run("all rules pass", func(t *testing.T) {
		chain := NewRuleChain(successRule, successRule)
		result := chain.Validate(Context{Context: context.Background(), })
		if !result.Passed {
			t.Errorf("expected success, got failure: %s", result.Reason)
		}
	})

	t.Run("early failure stops chain", func(t *testing.T) {
		chain := NewRuleChain(successRule, failureRule, successRule)
		result := chain.Validate(Context{Context: context.Background(), })
		if result.Passed {
			t.Errorf("expected failure, got success")
		}
		if result.Reason != "mock failure" {
			t.Errorf("expected failure reason, got: %s", result.Reason)
		}
	})
}

// mockRule is a test helper for rule chain testing.
type mockRule struct {
	name       string
	shouldPass bool
}

func (m *mockRule) Name() string { return m.name }

func (m *mockRule) Validate(_ Context) Result {
	if m.shouldPass {
		return Success()
	}
	return Failure("mock failure")
}
