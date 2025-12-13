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
	"golang.org/x/net/html"
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

// RenderedSample represents an HTML element to verify in rendered output.
type RenderedSample struct {
	File         string            `json:"file"`                   // Relative path to HTML file in public/
	Selector     string            `json:"selector"`               // CSS-like selector (simplified)
	ExpectCount  int               `json:"expectCount"`            // Expected number of matches (0 = must not exist)
	ContainsText string            `json:"containsText,omitempty"` // Text that should be present
	Attributes   map[string]string `json:"attributes,omitempty"`   // Expected attributes
}

// RenderedSamples is a collection of HTML verification samples.
type RenderedSamples struct {
	Samples []RenderedSample `json:"samples"`
}

// verifyRenderedSamples verifies specific HTML elements in rendered Hugo output.
// This uses golang.org/x/net/html to parse and validate DOM structure.
func verifyRenderedSamples(t *testing.T, outputDir, goldenPath string, updateGolden bool) {
	t.Helper()

	publicDir := filepath.Join(outputDir, "public")

	// Check if public directory exists (Hugo must have rendered)
	if _, err := os.Stat(publicDir); os.IsNotExist(err) {
		t.Skip("Skipping HTML verification - public directory not found (Hugo rendering disabled)")
		return
	}

	if updateGolden {
		t.Skip("Skipping HTML verification in update-golden mode (requires manual golden file creation)")
		return
	}

	// Load golden samples
	goldenData, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "failed to read golden samples file")

	var samples RenderedSamples
	err = json.Unmarshal(goldenData, &samples)
	require.NoError(t, err, "failed to parse golden samples")

	// Verify each sample
	for _, sample := range samples.Samples {
		t.Run(sample.File+":"+sample.Selector, func(t *testing.T) {
			htmlPath := filepath.Join(publicDir, sample.File)

			// Read HTML file
			htmlFile, err := os.Open(htmlPath)
			require.NoError(t, err, "failed to open HTML file: %s", sample.File)
			defer htmlFile.Close()

			// Parse HTML
			doc, err := html.Parse(htmlFile)
			require.NoError(t, err, "failed to parse HTML: %s", sample.File)

			// Find matching elements
			matches := findElements(doc, sample.Selector)

			// Verify count
			require.Equal(t, sample.ExpectCount, len(matches),
				"selector %s in %s: expected %d matches, got %d",
				sample.Selector, sample.File, sample.ExpectCount, len(matches))

			// Verify text content if specified
			if sample.ContainsText != "" && len(matches) > 0 {
				found := false
				for _, node := range matches {
					if containsText(node, sample.ContainsText) {
						found = true
						break
					}
				}
				require.True(t, found,
					"selector %s in %s: expected to contain text %q",
					sample.Selector, sample.File, sample.ContainsText)
			}

			// Verify attributes if specified
			if len(sample.Attributes) > 0 && len(matches) > 0 {
				for attrName, expectedValue := range sample.Attributes {
					found := false
					for _, node := range matches {
						if getAttr(node, attrName) == expectedValue {
							found = true
							break
						}
					}
					require.True(t, found,
						"selector %s in %s: expected attribute %s=%q",
						sample.Selector, sample.File, attrName, expectedValue)
				}
			}
		})
	}
}

// findElements searches for elements matching a simplified CSS selector.
// Supports: tag, .class, #id, tag.class, tag#id
func findElements(n *html.Node, selector string) []*html.Node {
	var results []*html.Node

	// Parse simple selector
	tag, class, id := parseSimpleSelector(selector)

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			match := true

			// Check tag
			if tag != "" && node.Data != tag {
				match = false
			}

			// Check class
			if match && class != "" {
				classAttr := getAttr(node, "class")
				if !strings.Contains(" "+classAttr+" ", " "+class+" ") {
					match = false
				}
			}

			// Check id
			if match && id != "" {
				if getAttr(node, "id") != id {
					match = false
				}
			}

			if match {
				results = append(results, node)
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(n)
	return results
}

// parseSimpleSelector parses a simplified CSS selector into tag, class, and id.
// Examples: "div", "div.container", "#main", "article#post"
func parseSimpleSelector(selector string) (tag, class, id string) {
	// Find # for id
	if idx := strings.Index(selector, "#"); idx >= 0 {
		tag = selector[:idx]
		id = selector[idx+1:]
		return
	}

	// Find . for class
	if idx := strings.Index(selector, "."); idx >= 0 {
		tag = selector[:idx]
		class = selector[idx+1:]
		return
	}

	// Just tag
	tag = selector
	return
}

// getAttr retrieves an attribute value from an HTML node.
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// containsText checks if a node or its descendants contain the specified text.
func containsText(n *html.Node, text string) bool {
	if n.Type == html.TextNode {
		return strings.Contains(strings.ToLower(n.Data), strings.ToLower(text))
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if containsText(child, text) {
			return true
		}
	}

	return false
}

// renderHugoSite runs `hugo` command to generate static HTML from the Hugo site.
// Returns the path to the public directory.
func renderHugoSite(t *testing.T, hugoSiteDir string) string {
	t.Helper()

	// Check if hugo is available
	if _, err := exec.LookPath("hugo"); err != nil {
		t.Skip("Hugo not found in PATH, skipping HTML rendering tests")
		return ""
	}

	// Run hugo build
	cmd := exec.Command("hugo", "--quiet")
	cmd.Dir = hugoSiteDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Hugo build output:\n%s", output)
		require.NoError(t, err, "hugo build failed")
	}

	publicDir := filepath.Join(hugoSiteDir, "public")
	require.DirExists(t, publicDir, "public directory should exist after hugo build")

	return publicDir
}
