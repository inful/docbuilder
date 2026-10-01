package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// TestBuildPath_BuildServiceMatchesDirectGenerator is a regression canary.
// It proves that the content tree produced by build.BuildService.Run is
// byte-identical to the one produced by calling hugo.Generator.GenerateFullSite
// directly. Both paths construct the same generator today, but if a future
// refactor ever introduces divergence (extra transforms, the Write path
// skipping the report generator, etc.), this test catches it.
//
// The diff focuses on .md files within the content tree. It does not compare
// generated metadata (build-report.json) or Hugo-rendered artifacts (public/),
// only the source content the content pipeline produced.
//
// Originally added as a refactor scaffolding test for the build-service-
// unification work. Kept as a permanent regression canary because the
// BuildService / direct-generator equivalence is the central invariant of
// that refactor.
func TestBuildPath_BuildServiceMatchesDirectGenerator(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build-path parity test in -short mode")
	}

	const (
		fixtureRepo = "../../test/testdata/repos/transforms/frontmatter-injection"
		fixtureCfg  = "../../test/testdata/configs/frontmatter-injection.yaml"
	)

	repoPath := setupTestRepo(t, fixtureRepo)
	baseCfg := loadGoldenConfig(t, fixtureCfg)
	require.Len(t, baseCfg.Repositories, 1, "fixture must have exactly one repository")
	baseCfg.Repositories[0].URL = repoPath

	// Two independent configs so neither path mutates the other's output
	// directory or workspace.
	cfgSvc := *baseCfg
	cfgSvc.Output.Directory = t.TempDir()

	cfgDirect := *baseCfg
	cfgDirect.Output.Directory = t.TempDir()

	// Path A — BuildService (the post-refactor CLI path).
	svc := build.NewBuildService()
	result, err := svc.Run(t.Context(), build.BuildRequest{
		Config:    &cfgSvc,
		OutputDir: cfgSvc.Output.Directory,
	})
	require.NoError(t, err, "BuildService.Run failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status)

	// Path B — direct hugo.Generator (today's CLI path).
	ws := workspace.NewManager("")
	require.NoError(t, ws.Create(), "workspace create failed")
	t.Cleanup(func() { _ = ws.Cleanup() })

	gen := hugo.NewGenerator(&cfgDirect, cfgDirect.Output.Directory)
	_, err = gen.GenerateFullSite(t.Context(), cfgDirect.Repositories, ws.GetPath())
	require.NoError(t, err, "hugo.Generator.GenerateFullSite failed")

	// Both paths must produce the same content tree.
	requireContentTreesEqual(t,
		filepath.Join(cfgSvc.Output.Directory, "content"),
		filepath.Join(cfgDirect.Output.Directory, "content"),
	)
}

// requireContentTreesEqual asserts that two content directories contain the same
// files with the same contents, regardless of order.
func requireContentTreesEqual(t *testing.T, a, b string) {
	t.Helper()

	if err := filepath.WalkDir(a, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(a, path)
		if err != nil {
			return err
		}

		other := filepath.Join(b, rel)
		otherInfo, err := os.Stat(other)
		if err != nil {
			t.Fatalf("file %s exists in A but not B", rel)
			return nil
		}
		if otherInfo.IsDir() {
			t.Fatalf("A file %s has type mismatch (dir) in B", rel)
			return nil
		}

		aBytes, err := os.ReadFile(path) //nolint:gosec // test reads within controlled tempdir
		if err != nil {
			return err
		}
		bBytes, err := os.ReadFile(other) //nolint:gosec // test reads within controlled tempdir
		if err != nil {
			return err
		}
		if !bytesEqual(aBytes, bBytes) {
			t.Fatalf("content mismatch for %s\n--- A ---\n%s\n--- B ---\n%s",
				rel, truncate(string(aBytes), 400), truncate(string(bBytes), 400))
		}
		return nil
	}); err != nil {
		t.Fatalf("walk A: %v", err)
	}

	// Ensure B has no files that A doesn't have (asymmetry check).
	if err := filepath.WalkDir(b, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(b, path)
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(a, rel)); err != nil {
			t.Fatalf("file %s exists in B but not A", rel)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk B: %v", err)
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}
