package integration

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ContentStructure represents the structure of generated content for golden testing.
type ContentStructure struct {
	Files     map[string]ContentFile `json:"files"`
	Structure map[string]interface{} `json:"structure"`
}

// ContentFile represents a single content file with its front matter and hash.
type ContentFile struct {
	FrontMatter map[string]interface{} `json:"frontmatter"`
	ContentHash string                 `json:"contentHash"`
}

// setupTestRepo creates a temporary git repository from a directory structure.
// The repository is initialized with an initial commit containing all files.
func setupTestRepo(t *testing.T, repoPath string) string {
	t.Helper()

	tmpDir := t.TempDir()

	err := copyDir(repoPath, tmpDir)
	require.NoError(t, err, "failed to copy test repo files")

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run(), "failed to initialize git repo")

	configUser := exec.Command("git", "-C", tmpDir, "config", "user.name", "Test")
	require.NoError(t, configUser.Run(), "failed to configure git user.name")

	configEmail := exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com")
	require.NoError(t, configEmail.Run(), "failed to configure git user.email")

	addCmd := exec.Command("git", "-C", tmpDir, "add", ".")
	require.NoError(t, addCmd.Run(), "failed to add files to git")

	commitCmd := exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial test commit")
	require.NoError(t, commitCmd.Run(), "failed to create initial commit")

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
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

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
	actualData, err := os.ReadFile(actualPath)
	require.NoError(t, err, "failed to read generated hugo.yaml")

	var actual map[string]interface{}
	err = yaml.Unmarshal(actualData, &actual)
	require.NoError(t, err, "failed to parse hugo.yaml")

	normalizeDynamicFields(actual)

	if updateGolden {
		data, err := yaml.Marshal(actual)
		require.NoError(t, err, "failed to marshal golden config")

		err = os.MkdirAll(filepath.Dir(goldenPath), 0755)
		require.NoError(t, err, "failed to create golden directory")

		err = os.WriteFile(goldenPath, data, 0644)
		require.NoError(t, err, "failed to write golden file")

		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	goldenData, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file: %s", goldenPath)

	var expected map[string]interface{}
	err = yaml.Unmarshal(goldenData, &expected)
	require.NoError(t, err, "failed to parse golden config")

	actualJSON, _ := json.MarshalIndent(actual, "", "  ")
	expectedJSON, _ := json.MarshalIndent(expected, "", "  ")

	require.JSONEq(t, string(expectedJSON), string(actualJSON), "Hugo config mismatch")
}

// normalizeDynamicFields removes or normalizes fields that change between runs.
func normalizeDynamicFields(cfg map[string]interface{}) {
	delete(cfg, "build_date")

	if params, ok := cfg["params"].(map[string]interface{}); ok {
		delete(params, "build_date")
	}
}

// normalizeFrontMatter removes dynamic fields from front matter.
func normalizeFrontMatter(fm map[string]interface{}) {
	if fm == nil {
		return
	}
	// Remove timestamp fields that change between runs
	delete(fm, "date")
	delete(fm, "lastmod")
	delete(fm, "publishDate")
	delete(fm, "expiryDate")
	
	// Remove editURL if it contains /tmp/ paths (dynamic test paths)
	if editURL, ok := fm["editURL"].(string); ok && strings.Contains(editURL, "/tmp/") {
		delete(fm, "editURL")
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
		Structure: make(map[string]interface{}),
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
		data, err := json.MarshalIndent(actual, "", "  ")
		require.NoError(t, err, "failed to marshal content structure")

		err = os.MkdirAll(filepath.Dir(goldenPath), 0755)
		require.NoError(t, err, "failed to create golden directory")

		err = os.WriteFile(goldenPath, data, 0644)
		require.NoError(t, err, "failed to write golden file")

		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	goldenData, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden file: %s", goldenPath)

	var expected ContentStructure
	err = json.Unmarshal(goldenData, &expected)
	require.NoError(t, err, "failed to parse golden content structure")

	actualJSON, _ := json.MarshalIndent(actual, "", "  ")
	expectedJSON, _ := json.MarshalIndent(expected, "", "  ")

	// If content structures don't match, provide detailed diff of mismatched files
	if string(expectedJSON) != string(actualJSON) {
		dumpContentDiff(t, outputDir, expected, *actual)
	}

	require.JSONEq(t, string(expectedJSON), string(actualJSON), "Content structure mismatch")
}

// parseFrontMatter extracts YAML front matter from markdown content.
func parseFrontMatter(data []byte) (map[string]interface{}, []byte) {
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

	var fm map[string]interface{}
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
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	t.Logf("Files with content hash mismatches: %d", len(sorted))
	
	for _, path := range sorted {
		fullPath := filepath.Join(outputDir, path)
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
		os.WriteFile(debugPath, body, 0644)
		t.Logf("Wrote body to: %s", debugPath)
	}
}

// buildStructureTree creates a nested map representing the directory structure.
func buildStructureTree(rootDir string) map[string]interface{} {
	tree := make(map[string]interface{})

	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == rootDir {
			return err
		}

		relPath, _ := filepath.Rel(rootDir, path)
		parts := strings.Split(relPath, string(filepath.Separator))

		current := tree
		for i, part := range parts {
			if i == len(parts)-1 {
				if info.IsDir() {
					if _, exists := current[part]; !exists {
						current[part] = make(map[string]interface{})
					}
				} else {
					current[part] = map[string]interface{}{}
				}
			} else {
				if _, exists := current[part]; !exists {
					current[part] = make(map[string]interface{})
				}
				current = current[part].(map[string]interface{})
			}
		}

		return nil
	})

	return tree
}
