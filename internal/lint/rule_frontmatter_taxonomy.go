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

	// Categories must match: starts with uppercase, can contain uppercase/lowercase/numbers/spaces/hyphens/underscores.
	// But should not be all uppercase (checked separately).
	validCategoryPattern = regexp.MustCompile(`^[A-Z][a-zA-Z0-9 \-_]*$`)
	
	// All caps pattern to detect categories that are entirely uppercase (except numbers/separators).
	allCapsPattern = regexp.MustCompile(`^[A-Z0-9 \-_]+$`)
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

// isValidCategory checks if a category is valid.
// Valid categories:
// - Start with uppercase letter
// - Can contain uppercase/lowercase letters, numbers, spaces, hyphens, underscores
// - Must not be ALL_CAPS (e.g., "TUTORIAL" or "API REFERENCE" are invalid)
// - Mixed case with multiple capitals is OK (e.g., "API Guide" or "REST API" are valid if properly cased)
func isValidCategory(category string) bool {
	// Must match the basic pattern
	if !validCategoryPattern.MatchString(category) {
		return false
	}
	
	// Check if it's all caps (not allowed)
	// A category is all caps if all letters are uppercase (no lowercase letters)
	if allCapsPattern.MatchString(category) {
		// Check if there's at least one lowercase letter
		hasLowercase := false
		for _, r := range category {
			if unicode.IsLower(r) {
				hasLowercase = true
				break
			}
		}
		// If there are no lowercase letters and there's at least one uppercase letter, it's all caps
		if !hasLowercase {
			hasUppercase := false
			for _, r := range category {
				if unicode.IsUpper(r) {
					hasUppercase = true
					break
				}
			}
			if hasUppercase {
				return false // All caps (single or multiple words), invalid
			}
		}
	}
	
	return true
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
			"Categories must match the pattern: ^[A-Z][a-zA-Z0-9 \\-_]*$",
			"",
			"Invalid category: " + category,
			"Suggested:       " + suggested,
			"",
			"Rules:",
			"  • Must start with an uppercase letter (A-Z)",
			"  • Can contain uppercase and lowercase letters",
			"  • Numbers are allowed (0-9)",
			"  • Spaces, hyphens (-) and underscores (_) are allowed",
			"  • Must not be ALL_CAPS (e.g., 'TUTORIAL' → 'Tutorial')",
			"  • Multiple capitals are OK (e.g., 'API Guide' is valid)",
			"  • No other special characters",
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

// normalizeCategory converts a category to valid format.
// Handles ALL_CAPS by capitalizing first letter and lowercasing the rest.
// Preserves spaces and mixed case like "API Guide".
func normalizeCategory(category string) string {
	// Remove any invalid characters (keep only letters, numbers, spaces, -, _)
	result := regexp.MustCompile(`[^a-zA-Z0-9 \-_]`).ReplaceAllString(category, "")
	
	if len(result) == 0 {
		return result
	}
	
	// Check if it's all caps (needs normalization)
	isAllCaps := true
	hasLetter := false
	for _, r := range result {
		if unicode.IsLetter(r) {
			hasLetter = true
			if unicode.IsLower(r) {
				isAllCaps = false
				break
			}
		}
	}
	
	// If it's all caps and has letters, convert to title case (first letter upper, rest lower)
	if isAllCaps && hasLetter {
		runes := []rune(result)
		runes[0] = unicode.ToUpper(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
		return string(runes)
	}
	
	// Otherwise, just ensure first letter is uppercase
	runes := []rune(result)
	runes[0] = unicode.ToUpper(runes[0])
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
