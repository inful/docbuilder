package lint

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// FrontmatterTaxonomyRule validates tags and categories in frontmatter.
type FrontmatterTaxonomyRule struct{}

const (
	frontmatterTaxonomyRuleName = "frontmatter-taxonomy"
)

var (
	// Tags must match: lowercase letters, numbers, hyphens, underscores.
	validTagPattern = regexp.MustCompile(`^[a-z0-9\-_]*$`)

	// Categories must match: starts with uppercase, followed by lowercase/numbers/hyphens/underscores.
	validCategoryPattern = regexp.MustCompile(`^[A-Z][a-z0-9\-_]*$`)
)

// Name returns the rule identifier.
func (r *FrontmatterTaxonomyRule) Name() string {
	return frontmatterTaxonomyRuleName
}

// AppliesTo returns true for markdown documentation files.
func (r *FrontmatterTaxonomyRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// Check validates tags and categories in the file's frontmatter.
func (r *FrontmatterTaxonomyRule) Check(filePath string) ([]Issue, error) {
	// #nosec G304 -- filePath is derived from the current lint target.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	fm, ok := extractFrontmatter(string(data))
	if !ok {
		// No frontmatter - nothing to validate
		return nil, nil
	}

	var obj map[string]any
	if err := yaml.Unmarshal([]byte(fm), &obj); err != nil {
		// Invalid YAML - other rules may report it
		return nil, nil //nolint:nilerr
	}

	var issues []Issue

	// Validate tags
	if tagsAny, hasTags := obj["tags"]; hasTags {
		tags := extractStringArray(tagsAny)
		for _, tag := range tags {
			if !isValidTag(tag) {
				issues = append(issues, r.invalidTagIssue(filePath, tag))
			}
		}
	}

	// Validate categories
	if categoriesAny, hasCategories := obj["categories"]; hasCategories {
		categories := extractStringArray(categoriesAny)
		for _, category := range categories {
			if !isValidCategory(category) {
				issues = append(issues, r.invalidCategoryIssue(filePath, category))
			}
		}
	}

	return issues, nil
}

// isValidTag checks if a tag matches the pattern ^[a-z0-9\-_]*$.
func isValidTag(tag string) bool {
	return validTagPattern.MatchString(tag)
}

// isValidCategory checks if a category matches the pattern ^[A-Z][a-z0-9\-_]*$.
func isValidCategory(category string) bool {
	return validCategoryPattern.MatchString(category)
}

// extractStringArray converts various YAML array formats to []string.
func extractStringArray(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []any:
		var result []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return v
	default:
		return nil
	}
}

func (r *FrontmatterTaxonomyRule) invalidTagIssue(filePath, tag string) Issue {
	suggested := normalizeTag(tag)
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     frontmatterTaxonomyRuleName,
		Message:  fmt.Sprintf("Invalid tag format: %q", tag),
		Explanation: strings.TrimSpace(strings.Join([]string{
			"Tags must match the pattern: ^[a-z0-9\\-_]*$",
			"",
			"Invalid tag: " + tag,
			"Suggested:   " + suggested,
			"",
			"Rules:",
			"  • Must contain only lowercase letters (a-z)",
			"  • Numbers are allowed (0-9)",
			"  • Hyphens (-) and underscores (_) are allowed",
			"  • No uppercase letters",
			"  • No spaces (use underscores instead)",
			"  • No special characters",
		}, "\n")),
		Fix:  "Run: docbuilder lint --fix (automatically normalizes tags)",
		Line: 0,
	}
}

func (r *FrontmatterTaxonomyRule) invalidCategoryIssue(filePath, category string) Issue {
	suggested := normalizeCategory(category)
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     frontmatterTaxonomyRuleName,
		Message:  fmt.Sprintf("Invalid category format: %q", category),
		Explanation: strings.TrimSpace(strings.Join([]string{
			"Categories must match the pattern: ^[A-Z][a-z0-9\\-_]*$",
			"",
			"Invalid category: " + category,
			"Suggested:       " + suggested,
			"",
			"Rules:",
			"  • Must start with an uppercase letter (A-Z)",
			"  • Followed by lowercase letters, numbers, hyphens, or underscores",
			"  • Numbers are allowed (0-9)",
			"  • Hyphens (-) and underscores (_) are allowed",
			"  • No spaces (use underscores instead)",
			"  • No special characters",
		}, "\n")),
		Fix:  "Run: docbuilder lint --fix (automatically normalizes categories)",
		Line: 0,
	}
}

// normalizeTag converts a tag to valid format: lowercase, spaces to underscores.
func normalizeTag(tag string) string {
	// Convert to lowercase
	result := strings.ToLower(tag)
	// Replace spaces with underscores
	result = strings.ReplaceAll(result, " ", "_")
	// Remove any invalid characters (keep only a-z, 0-9, -, _)
	result = regexp.MustCompile(`[^a-z0-9\-_]`).ReplaceAllString(result, "")
	return result
}

// normalizeCategory converts a category to valid format: capitalize first letter, lowercase rest, spaces to underscores.
func normalizeCategory(category string) string {
	// Replace spaces with underscores first
	result := strings.ReplaceAll(category, " ", "_")
	// Remove any invalid characters (keep only letters, numbers, -, _)
	result = regexp.MustCompile(`[^a-zA-Z0-9\-_]`).ReplaceAllString(result, "")

	// Capitalize: first letter uppercase, rest lowercase
	if len(result) == 0 {
		return result
	}

	runes := []rune(result)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}

	return string(runes)
}

// normalizeTags normalizes an array of tags and returns whether any changes were made.
func normalizeTags(tags []string) ([]string, bool) {
	if len(tags) == 0 {
		return tags, false
	}

	result := make([]string, len(tags))
	changed := false

	for i, tag := range tags {
		normalized := normalizeTag(tag)
		result[i] = normalized
		if normalized != tag {
			changed = true
		}
	}

	return result, changed
}

// normalizeCategories normalizes an array of categories and returns whether any changes were made.
func normalizeCategories(categories []string) ([]string, bool) {
	if len(categories) == 0 {
		return categories, false
	}

	result := make([]string, len(categories))
	changed := false

	for i, category := range categories {
		normalized := normalizeCategory(category)
		result[i] = normalized
		if normalized != category {
			changed = true
		}
	}

	return result, changed
}
