package lint

import (
	"os"
	"regexp"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// CategoryNamingRule reports an ERROR when a category doesn't follow
// the project's kebab-case convention.
//
// Acceptable form: ^[a-z][a-z0-9-]*$ (lowercase start, lowercase
// letters / digits / hyphens thereafter).
//
// The lint is strict — ERROR severity, no auto-fix. Renaming a
// category changes URLs (the taxonomy is part of the rendered URL
// path on Hugo), so the fix requires a manual decision and possibly
// alias updates.
type CategoryNamingRule struct{}

// categoryPattern is the kebab-case matcher.
var categoryPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Name returns the rule identifier.
func (r *CategoryNamingRule) Name() string {
	return ruleCategoryNaming
}

// AppliesTo returns true for documentation files.
func (r *CategoryNamingRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// Check reads frontmatter and flags any non-kebab-case category.
func (r *CategoryNamingRule) Check(filePath string) ([]Issue, error) {
	// #nosec G304 -- filePath is from controlled lint walk.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	fmBytes, _, had, _, splitErr := frontmatter.Split(data)
	if splitErr != nil || !had {
		return nil, nil
	}
	fields, parseErr := frontmatter.ParseYAML(fmBytes)
	if parseErr != nil {
		return nil, nil
	}
	catsAny, ok := fields["categories"]
	if !ok {
		return nil, nil
	}

	var issues []Issue
	for _, cat := range listStrings(catsAny) {
		if !categoryPattern.MatchString(cat) {
			issues = append(issues, r.badCategoryIssue(filePath, cat))
		}
	}
	return issues, nil
}

// listStrings coerces a YAML list value to a slice of strings. Used
// for the `categories` field which may be `[]any` or `[]string` after
// frontmatter parsing. Non-string items are stringified.
func listStrings(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return x
	}
	return nil
}

func (r *CategoryNamingRule) badCategoryIssue(filePath, cat string) Issue {
	suggested := toKebabCase(cat)
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "Category must be kebab-case",
		Explanation: "Category `" + cat + "` does not match the project's kebab-case convention. " +
			"Every other category in the project is lowercase (how-to, reference, explanation, " +
			"security, architecture-decisions, tutorials, ci-cd, documentation, development). " +
			"A single capitalized entry looks like a typo and breaks the visual rhythm of the sidebar.",
		Fix: "Rename to `" + suggested + "`. Note: renaming a category changes URLs; review " +
			"existing cross-links and add aliases if needed.",
		Line: 0,
	}
}

// toKebabCase lowercases the first character and replaces underscores
// with hyphens. Used only as a hint, not as an automatic fix.
func toKebabCase(s string) string {
	if s == "" {
		return s
	}
	out := []byte(s)
	if out[0] >= 'A' && out[0] <= 'Z' {
		out[0] += 'a' - 'A'
	}
	for i, b := range out {
		if b == '_' {
			out[i] = '-'
		}
	}
	return string(out)
}

// listStrings is defined earlier in this file.
