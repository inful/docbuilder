package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// ContentStructure represents the structure of generated content for golden testing.
type ContentStructure struct {
	Files     map[string]ContentFile `json:"files"`
	Structure map[string]any         `json:"structure"`
}

// ContentFile represents a single content file with its front matter and hash.
type ContentFile struct {
	FrontMatter map[string]any `json:"frontmatter"`
	ContentHash string         `json:"contentHash"`
}

// setupTestRepo creates a temporary git repository from a directory structure.
// The repository is initialized with an initial commit containing all files.
func setupTestRepo(t *testing.T, repoPath string) string {
	t.Helper()

	tmpDir := t.TempDir()

	err := copyDir(repoPath, tmpDir)
	require.NoError(t, err, "failed to copy test repo files")

	// Initialize git repository using go-git
	repo, err := git.PlainInit(tmpDir, false)
	require.NoError(t, err, "failed to initialize git repo")

	// Get the worktree
	w, err := repo.Worktree()
	require.NoError(t, err, "failed to get worktree")

	// Add all files
	err = w.AddGlob(".")
	require.NoError(t, err, "failed to add files to git")

	// Create initial commit
	_, err = w.Commit("Initial test commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err, "failed to create initial commit")

	// Ensure branch is named 'main' for consistency with test configs
	// go-git creates a branch based on Git's default, so we rename if needed
	headRef, err := repo.Head()
	require.NoError(t, err, "failed to get HEAD")

	if headRef.Name().Short() != "main" {
		// Create 'main' branch pointing to current HEAD
		mainRef := headRef.Name()
		err = w.Checkout(&git.CheckoutOptions{
			Branch: "refs/heads/main",
			Create: true,
		})
		require.NoError(t, err, "failed to create main branch")

		// Delete the old default branch if it's not main
		if mainRef.Short() != "main" {
			_ = repo.Storer.RemoveReference(mainRef)
			// Ignore error if reference doesn't exist
		}
	}

	return tmpDir
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		if strings.Contains(relPath, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath)
	})
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	// #nosec G304 -- test utility with paths from test setup, not user input
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	// #nosec G304 -- test utility with paths from test setup, not user input
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// loadGoldenConfig loads a test configuration and returns it.
func loadGoldenConfig(t *testing.T, configPath string) *config.Config {
	t.Helper()

	cfg, err := config.Load(configPath)
	require.NoError(t, err, "failed to load test config")

	return cfg
}

// verifyHugoConfig compares the generated hugo.yaml against a golden file.
func verifyHugoConfig(t *testing.T, outputDir, goldenPath string, updateGolden bool) {
	t.Helper()

	actualPath := filepath.Join(outputDir, "hugo.yaml")
	// #nosec G304 -- test utility reading from test output directory
	actualData, err := os.ReadFile(actualPath)
	require.NoError(t, err, "failed to read generated hugo.yaml")

	var actual map[string]any
	err = yaml.Unmarshal(actualData, &actual)
	require.NoError(t, err, "failed to parse hugo.yaml")

	normalizeDynamicFields(actual)

	if updateGolden {
		var data []byte
		data, err = yaml.Marshal(actual)
		require.NoError(t, err, "failed to marshal golden config")

		err = os.MkdirAll(filepath.Dir(goldenPath), 0o750)
		require.NoError(t, err, "failed to create golden directory")

		err = os.WriteFile(goldenPath, data, 0o600)
		require.NoError(t, err, "failed to write golden file")

		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// #nosec G304 -- test utility reading golden file from testdata
	goldenData, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file: %s", goldenPath)

	var expected map[string]any
	err = yaml.Unmarshal(goldenData, &expected)
	require.NoError(t, err, "failed to parse golden config")

	actualJSON, marshalErr := json.MarshalIndent(actual, "", "  ")
	if marshalErr != nil {
		t.Fatalf("failed to marshal actual content structure: %v", marshalErr)
	}
	expectedJSON, marshalErr := json.MarshalIndent(expected, "", "  ")
	if marshalErr != nil {
		t.Fatalf("failed to marshal expected content structure: %v", marshalErr)
	}

	require.JSONEq(t, string(expectedJSON), string(actualJSON), "Hugo config mismatch")
}

// normalizeDynamicFields removes or normalizes fields that change between runs.
func normalizeDynamicFields(cfg map[string]any) {
	delete(cfg, "build_date")

	if params, ok := cfg["params"].(map[string]any); ok {
		delete(params, "build_date")
	}
}

// normalizeFrontMatter normalizes front matter to keep only structural keys.
// This ensures golden tests focus on site structure, not generated metadata.
// Structural keys (kept): title, linkTitle, weight, type, slug, url, cascade, menu.
// All other keys (dropped): date, lastmod, fingerprint, uid, description, editURL, etc.
func normalizeFrontMatter(fm map[string]any) {
	if fm == nil {
		return
	}

	// Whitelist of structural keys that define site hierarchy and navigation.
	structuralKeys := map[string]bool{
		"title":     true,
		"linkTitle": true,
		"weight":    true,
		"type":      true,
		"slug":      true,
		"url":       true,
		"cascade":   true,
		"menu":      true,
		"public":    true, // Filter applied by daemon mode
	}

	// Keep only structural keys; delete all others (metadata, timestamps, hashes, etc.)
	keysToDelete := []string{}
	for key := range fm {
		if !structuralKeys[key] {
			keysToDelete = append(keysToDelete, key)
		}
	}
	for _, key := range keysToDelete {
		delete(fm, key)
	}
}

// normalizeBodyContent removes or replaces dynamic content from markdown body.
// This ensures golden tests are reproducible even when file paths change.
func normalizeBodyContent(body []byte) []byte {
	content := string(body)

	// Replace /tmp/TestGolden_*/NNN paths with normalized placeholders
	// Pattern: /tmp/TestGolden_Something123456789/001
	re := regexp.MustCompile(`/tmp/TestGolden_[^/]+/\d+`)
	content = re.ReplaceAllString(content, "/tmp/test-repo")

	return []byte(content)
}

// verifyContentStructure compares the generated content structure against a golden file.
func verifyContentStructure(t *testing.T, outputDir, goldenPath string, updateGolden bool) {
	t.Helper()

	contentDir := filepath.Join(outputDir, "content")

	actual := &ContentStructure{
		Files:     make(map[string]ContentFile),
		Structure: make(map[string]any),
	}

	err := filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		relPath, _ := filepath.Rel(outputDir, path)

		// #nosec G304 -- test utility reading from test output directory
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		fm, content := parseFrontMatter(data)

		// Normalize dynamic fields in front matter
		normalizeFrontMatter(fm)

		// Normalize dynamic content in body (e.g., temp paths)
		normalizedContent := normalizeBodyContent(content)

		hash := sha256.Sum256(normalizedContent)

		actual.Files[relPath] = ContentFile{
			FrontMatter: fm,
			ContentHash: fmt.Sprintf("sha256:%x", hash[:8]),
		}

		return nil
	})
	require.NoError(t, err, "failed to walk content directory")

	actual.Structure = buildStructureTree(contentDir)

	if updateGolden {
		var data []byte
		data, err = json.MarshalIndent(actual, "", "  ")
		require.NoError(t, err, "failed to marshal content structure")

		err = os.MkdirAll(filepath.Dir(goldenPath), 0o750)
		require.NoError(t, err, "failed to create golden directory")

		err = os.WriteFile(goldenPath, data, 0o600)
		require.NoError(t, err, "failed to write golden file")

		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	goldenData, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file: %s", goldenPath)

	var expected ContentStructure
	err = json.Unmarshal(goldenData, &expected)
	require.NoError(t, err, "failed to parse golden content structure")

	actualJSON, marshalErr := json.MarshalIndent(actual, "", "  ")
	if marshalErr != nil {
		t.Fatalf("failed to marshal actual content structure: %v", marshalErr)
	}
	expectedJSON, marshalErr := json.MarshalIndent(expected, "", "  ")
	if marshalErr != nil {
		t.Fatalf("failed to marshal expected content structure: %v", marshalErr)
	}

	// If content structures don't match, provide detailed diff of mismatched files
	if !bytes.Equal(expectedJSON, actualJSON) {
		dumpContentDiff(t, outputDir, expected, *actual)
	}

	require.JSONEq(t, string(expectedJSON), string(actualJSON), "Content structure mismatch")
}

// parseFrontMatter extracts YAML front matter from markdown content.
func parseFrontMatter(data []byte) (map[string]any, []byte) {
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		return nil, data
	}

	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return nil, data
	}

	fmStr := content[4 : endIdx+4]
	bodyContent := content[endIdx+9:]

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(fmStr), &fm); err != nil {
		return nil, data
	}

	return fm, []byte(bodyContent)
}

// dumpContentDiff provides detailed debugging output when content structures differ.
// It compares files with hash mismatches and dumps the actual content for inspection.
func dumpContentDiff(t *testing.T, outputDir string, expected, actual ContentStructure) {
	t.Helper()

	// Find files with different content hashes
	diffFiles := []string{}
	for path, expectedFile := range expected.Files {
		if actualFile, ok := actual.Files[path]; ok {
			if expectedFile.ContentHash != actualFile.ContentHash {
				diffFiles = append(diffFiles, path)
			}
		}
	}

	if len(diffFiles) == 0 {
		t.Log("Content structures differ but no specific files have hash mismatches (structure change)")
		return
	}

	// Sort for deterministic output
	sorted := make([]string, len(diffFiles))
	copy(sorted, diffFiles)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	t.Logf("Files with content hash mismatches: %d", len(sorted))

	for _, path := range sorted {
		fullPath := filepath.Join(outputDir, path)
		// #nosec G304 -- test utility reading from test output directory for debugging
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Logf("  %s: (error reading: %v)", path, err)
			continue
		}

		fm, body := parseFrontMatter(content)

		expectedFile := expected.Files[path]
		actualFile := actual.Files[path]

		t.Logf("\n--- File: %s ---", path)
		t.Logf("Expected hash: %s", expectedFile.ContentHash)
		t.Logf("Actual hash:   %s", actualFile.ContentHash)

		// Show front matter
		if fm != nil {
			fmBytes, _ := yaml.Marshal(fm)
			t.Logf("Front matter:\n%s", string(fmBytes))
		}

		// Show body content (first 500 chars or full if smaller)
		bodyStr := string(body)
		t.Logf("Body content (%d bytes):\n%s", len(bodyStr), bodyStr)
		t.Logf("Body SHA256: %x", sha256.Sum256(body))

		// Write to /tmp for debugging
		debugPath := filepath.Join("/tmp", "golden-debug-"+filepath.Base(path))
		_ = os.WriteFile(debugPath, body, 0o600)
		t.Logf("Wrote body to: %s", debugPath)
	}
}

// buildStructureTree creates a nested map representing the directory structure.
func buildStructureTree(rootDir string) map[string]any {
	tree := make(map[string]any)

	_ = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == rootDir {
			return err
		}

		relPath, _ := filepath.Rel(rootDir, path)
		parts := strings.Split(relPath, string(filepath.Separator))

		addPathToTree(tree, parts, info.IsDir())
		return nil
	})

	return tree
}

// addPathToTree adds a file or directory path to the structure tree.
func addPathToTree(tree map[string]any, parts []string, isDir bool) {
	current := tree
	for i, part := range parts {
		if i == len(parts)-1 {
			addFinalPart(current, part, isDir)
		} else {
			current = ensureIntermediateDir(current, part)
		}
	}
}

// addFinalPart adds the final file or directory to the tree.
func addFinalPart(current map[string]any, part string, isDir bool) {
	if isDir {
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]any)
		}
	} else {
		current[part] = map[string]any{}
	}
}

// ensureIntermediateDir ensures an intermediate directory exists in the tree.
func ensureIntermediateDir(current map[string]any, part string) map[string]any {
	if _, exists := current[part]; !exists {
		current[part] = make(map[string]any)
	}
	return current[part].(map[string]any)
}

// runGoldenTest is a helper that executes a golden test with standard setup.
// It handles repository setup, configuration loading, build execution, and verification.
func runGoldenTest(t *testing.T, testRepoPath, configPath, goldenDirPath string, updateGolden bool) {
	t.Helper()

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, testRepoPath)

	// Load test configuration
	cfg := loadGoldenConfig(t, configPath)

	// Point configuration to temporary repository
	require.Len(t, cfg.Repositories, 1, "expected exactly one repository in config")
	cfg.Repositories[0].URL = repoPath

	// Create temporary output directory
	outputDir := t.TempDir()
	cfg.Output.Directory = outputDir

	// Execute build
	result, err := runBuildPipeline(t, cfg, outputDir)
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify outputs
	verifyHugoConfig(t, outputDir, goldenDirPath+"/hugo-config.golden.yaml", updateGolden)
	verifyContentStructure(t, outputDir, goldenDirPath+"/content-structure.golden.json", updateGolden)
}

// runBuildPipeline creates and runs the build service.
func runBuildPipeline(t *testing.T, cfg *config.Config, outputDir string) (*build.BuildResult, error) {
	t.Helper()

	svc := build.NewBuildService().
		WithHugoGeneratorFactory(func(c *config.Config, outDir string) build.HugoGenerator {
			return hugo.NewGenerator(c, outDir)
		})

	req := build.BuildRequest{
		Config:    cfg,
		OutputDir: outputDir,
	}

	return svc.Run(t.Context(), req)
}

// runMultiRepoGoldenTest is a helper for tests with multiple repositories.
// It handles setup for 2 repositories, configuration loading, build execution, and verification.
func runMultiRepoGoldenTest(t *testing.T, repo1Path, repo2Path, configPath, goldenDirPath string, updateGolden bool) {
	t.Helper()

	// Create temporary git repositories from testdata
	tmpRepo1 := setupTestRepo(t, repo1Path)
	tmpRepo2 := setupTestRepo(t, repo2Path)

	// Load test configuration
	cfg := loadGoldenConfig(t, configPath)

	// Point configuration to temporary repositories
	require.Len(t, cfg.Repositories, 2, "expected exactly two repositories in config")
	cfg.Repositories[0].URL = tmpRepo1
	cfg.Repositories[1].URL = tmpRepo2

	// Create temporary output directory
	outputDir := t.TempDir()
	cfg.Output.Directory = outputDir

	// Execute build
	result, err := runBuildPipeline(t, cfg, outputDir)
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify outputs
	verifyHugoConfig(t, outputDir, goldenDirPath+"/hugo-config.golden.yaml", updateGolden)
	verifyContentStructure(t, outputDir, goldenDirPath+"/content-structure.golden.json", updateGolden)
}
