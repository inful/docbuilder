package lint

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// FrontmatterRule checks that markdown files have proper frontmatter with required fields.
type FrontmatterRule struct{}

// Name returns the name of the rule.
func (r *FrontmatterRule) Name() string {
	return "frontmatter"
}

// AppliesTo checks if the rule applies to the given file path.
func (r *FrontmatterRule) AppliesTo(filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".md") ||
		strings.HasSuffix(strings.ToLower(filePath), ".markdown")
}

// Check validates that the file has frontmatter with required fields.
func (r *FrontmatterRule) Check(filePath string) ([]Issue, error) {
	var issues []Issue

	//nolint:gosec // G304: Reading file by path is expected for a linter
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	hasFM := hasFrontmatter(string(content))
	if !hasFM {
		issues = append(issues, Issue{
			FilePath:    filePath,
			Severity:    SeverityWarning,
			Rule:        r.Name(),
			Message:     "Missing frontmatter",
			Explanation: "Markdown files should have YAML frontmatter with required fields (tags, categories, id)",
			Fix:         "Add frontmatter with tags: [], categories: [], and id fields",
			Line:        1,
		})
		return issues, nil
	}

	// Parse frontmatter
	fm, err := parseFrontmatterFromFile(string(content))
	if err != nil {
		issues = append(issues, Issue{
			FilePath:    filePath,
			Severity:    SeverityError,
			Rule:        r.Name(),
			Message:     fmt.Sprintf("Invalid frontmatter: %v", err),
			Explanation: "Frontmatter must be valid YAML between --- delimiters",
			Fix:         "Fix YAML syntax errors in frontmatter",
			Line:        1,
		})
		return issues, nil
	}

	// Check required fields
	if _, hasTags := fm["tags"]; !hasTags {
		issues = append(issues, Issue{
			FilePath:    filePath,
			Severity:    SeverityWarning,
			Rule:        r.Name(),
			Message:     "Missing 'tags' field in frontmatter",
			Explanation: "Frontmatter should include a 'tags' array for categorization",
			Fix:         "Add 'tags: []' to frontmatter",
			Line:        1,
		})
	}

	if _, hasCategories := fm["categories"]; !hasCategories {
		issues = append(issues, Issue{
			FilePath:    filePath,
			Severity:    SeverityWarning,
			Rule:        r.Name(),
			Message:     "Missing 'categories' field in frontmatter",
			Explanation: "Frontmatter should include a 'categories' array for organization",
			Fix:         "Add 'categories: []' to frontmatter",
			Line:        1,
		})
	}

	if _, hasID := fm["id"]; !hasID {
		issues = append(issues, Issue{
			FilePath:    filePath,
			Severity:    SeverityWarning,
			Rule:        r.Name(),
			Message:     "Missing 'id' field in frontmatter",
			Explanation: "Frontmatter should include a unique 'id' field (UUID)",
			Fix:         "Add 'id: <uuid>' to frontmatter",
			Line:        1,
		})
	}

	return issues, nil
}

// hasFrontmatter checks if content has YAML frontmatter block.
func hasFrontmatter(content string) bool {
	return strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n")
}

// parseFrontmatterFromFile extracts and parses YAML frontmatter from markdown content.
func parseFrontmatterFromFile(content string) (map[string]any, error) {
	if !hasFrontmatter(content) {
		return nil, errors.New("no frontmatter found")
	}

	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")

	// Find end delimiter
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return nil, errors.New("frontmatter too short")
	}

	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return nil, errors.New("frontmatter end delimiter not found")
	}

	// Extract YAML content
	yamlContent := strings.Join(lines[1:endIdx], "\n")

	// Parse YAML
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return fm, nil
}

// FixFrontmatter adds missing frontmatter or required fields to a markdown file.
func FixFrontmatter(filePath string) error {
	//nolint:gosec // G304: Reading file by path is expected for a linter
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)
	hasFM := hasFrontmatter(contentStr)

	// Get original file permissions
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	fileMode := fileInfo.Mode()

	if !hasFM {
		// Add complete frontmatter
		newID := uuid.New().String()
		frontmatter := fmt.Sprintf("---\ntags: []\ncategories: []\nid: %s\n---\n\n", newID)
		newContent := frontmatter + contentStr

		if errWrite := os.WriteFile(filePath, []byte(newContent), fileMode); errWrite != nil {
			return fmt.Errorf("failed to write file: %w", errWrite)
		}
		return nil
	}

	// Parse existing frontmatter
	fm, err := parseFrontmatterFromFile(contentStr)
	if err != nil {
		return fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Add missing fields
	modified := false

	if _, hasTags := fm["tags"]; !hasTags {
		fm["tags"] = []string{}
		modified = true
	}

	if _, hasCategories := fm["categories"]; !hasCategories {
		fm["categories"] = []string{}
		modified = true
	}

	if _, hasID := fm["id"]; !hasID {
		fm["id"] = uuid.New().String()
		modified = true
	}

	if !modified {
		return nil // Nothing to fix
	}

	// Rebuild content with updated frontmatter
	newContent, err := rebuildContentWithFrontmatter(contentStr, fm)
	if err != nil {
		return fmt.Errorf("failed to rebuild content: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(newContent), fileMode); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// rebuildContentWithFrontmatter replaces frontmatter in content with updated version.
// Uses yaml.Node to preserve field order from original frontmatter.
func rebuildContentWithFrontmatter(content string, fm map[string]any) (string, error) {
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")

	// Find end of frontmatter
	lines := strings.Split(content, "\n")
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return "", errors.New("frontmatter end delimiter not found")
	}

	// Extract original YAML content
	yamlContent := strings.Join(lines[1:endIdx], "\n")

	// Parse into Node to preserve field order
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &node); err != nil {
		return "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	// The node is typically a Document node containing a Mapping node
	var mappingNode *yaml.Node
	switch {
	case node.Kind == yaml.DocumentNode && len(node.Content) > 0:
		mappingNode = node.Content[0]
	case node.Kind == yaml.MappingNode:
		mappingNode = &node
	default:
		return "", errors.New("unexpected YAML structure")
	}

	if mappingNode.Kind != yaml.MappingNode {
		return "", errors.New("frontmatter is not a mapping")
	}

	// Update existing fields and add new ones while preserving order
	// Content in a MappingNode is [key1, value1, key2, value2, ...]
	existingKeys := make(map[string]int) // maps key to index in Content array
	for i := 0; i < len(mappingNode.Content); i += 2 {
		keyNode := mappingNode.Content[i]
		existingKeys[keyNode.Value] = i
	}

	// Update existing fields
	for key, value := range fm {
		if idx, exists := existingKeys[key]; exists {
			// Update the value node at idx+1
			valueNode := mappingNode.Content[idx+1]
			if err := valueNode.Encode(value); err != nil {
				return "", fmt.Errorf("failed to encode value for key %s: %w", key, err)
			}
		} else {
			// Add new field at the end
			keyNode := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
			}
			valueNode := &yaml.Node{}
			if err := valueNode.Encode(value); err != nil {
				return "", fmt.Errorf("failed to encode value for key %s: %w", key, err)
			}
			mappingNode.Content = append(mappingNode.Content, keyNode, valueNode)
		}
	}

	// Marshal back to YAML
	yamlBytes, err := yaml.Marshal(mappingNode)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Reconstruct content with two newlines after closing delimiter for consistency
	bodyContent := strings.Join(lines[endIdx+1:], "\n")
	// Trim leading newlines from body to avoid extra blank lines
	bodyContent = strings.TrimLeft(bodyContent, "\n")
	newContent := fmt.Sprintf("---\n%s---\n\n%s", string(yamlBytes), bodyContent)

	return newContent, nil
}
