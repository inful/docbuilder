package lint

import (
	"fmt"
	"os"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// FrontmatterRequiredFieldsRule verifies that every doc has the
// canonical frontmatter field set the build pipeline expects:
//
//   - title   (string)
//   - date    (RFC 3339 / ISO 8601 timestamp)
//   - lastmod (YYYY-MM-DD)
//   - categories (non-empty list)
//   - tags    (list; may be empty but the field must be present)
//
// `uid` and `fingerprint` are already lint-enforced separately.
//
// Severity:
//   - missing or empty values: ERROR (blocks build)
//   - malformed `date` or `lastmod`: WARNING (does not block)
type FrontmatterRequiredFieldsRule struct{}

// dateLayouts lists the accepted layouts for the `date` field. We
// accept strict RFC 3339 plus the looser forms commonly used in this
// repo's hand-edited frontmatter.
var dateLayouts = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05Z",
	"2006-01-02",
}

// lastmodLayout is the accepted layout for the `lastmod` field.
const lastmodLayout = "2006-01-02"

// Name returns the rule identifier.
func (r *FrontmatterRequiredFieldsRule) Name() string {
	return ruleFrontmatterRequiredFields
}

// AppliesTo returns true for documentation files.
func (r *FrontmatterRequiredFieldsRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// Check reads the frontmatter and reports one issue per missing,
// empty, or malformed field.
func (r *FrontmatterRequiredFieldsRule) Check(filePath string) ([]Issue, error) {
	// #nosec G304 -- filePath is from controlled lint walk.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	fmBytes, _, had, _, splitErr := frontmatter.Split(data)
	if splitErr != nil || !had {
		return []Issue{r.missingIssue(filePath, "frontmatter")}, nil
	}
	fields, parseErr := frontmatter.ParseYAML(fmBytes)
	if parseErr != nil {
		return []Issue{r.malformedYAMLError(filePath, parseErr)}, nil
	}

	var issues []Issue

	// title (string, non-empty)
	if title, ok := fields["title"].(string); !ok || strings.TrimSpace(title) == "" {
		issues = append(issues, r.missingIssue(filePath, "title"))
	}

	// date (RFC 3339 / ISO 8601)
	if v, ok := fields["date"]; !ok || v == nil || v == "" {
		issues = append(issues, r.missingIssue(filePath, "date"))
	} else if s, ok := dateStringValue(v); !ok || !parseAnyDate(s) {
		issues = append(issues, r.malformedDateIssue(filePath, s))
	}

	// lastmod (YYYY-MM-DD)
	if v, ok := fields["lastmod"]; !ok || v == nil || v == "" {
		issues = append(issues, r.missingIssue(filePath, "lastmod"))
	} else if s, ok := lastmodStringValue(v); !ok {
		issues = append(issues, r.malformedLastmodIssue(filePath, fmt.Sprintf("%v", v)))
	} else if _, err := time.Parse(lastmodLayout, s); err != nil {
		issues = append(issues, r.malformedLastmodIssue(filePath, s))
	}

	// categories (non-empty list)
	catsAny, ok := fields["categories"]
	if !ok || catsAny == nil {
		issues = append(issues, r.missingIssue(filePath, "categories"))
	} else {
		count := countListAny(catsAny)
		if count == 0 {
			issues = append(issues, r.emptyCategoriesIssue(filePath))
		}
	}

	// tags (list; may be empty but the field must be present)
	if _, ok := fields["tags"]; !ok {
		issues = append(issues, r.missingIssue(filePath, "tags"))
	}

	return issues, nil
}

// parseAnyDate returns true if the value parses as one of dateLayouts.
func parseAnyDate(s string) bool {
	for _, layout := range dateLayouts {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}

// dateStringValue coerces a YAML-parsed date value to its string form.
// YAML libraries often auto-parse RFC 3339 timestamps into `time.Time`
// values; this helper stringifies them so the layout check works the
// same regardless of YAML library version.
func dateStringValue(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case time.Time:
		return x.Format(time.RFC3339), true
	}
	return "", false
}

// lastmodStringValue coerces a YAML-parsed lastmod value to a string.
func lastmodStringValue(v any) (string, bool) {
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

func (r *FrontmatterRequiredFieldsRule) missingIssue(filePath, field string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "Missing required frontmatter field: " + field,
		Explanation: "Every doc must carry a `" + field + "` field in its YAML frontmatter. " +
			"`title` drives the rendered H1 and nav; `date` populates Hugo's date taxonomy; " +
			"`lastmod` is updated by lint_fix; `categories` drives the sidebar; `tags` " +
			"drives the /tags/ index pages.",
		Fix: "Add `" + field + ": <value>` to the frontmatter.",
		Line: 0,
	}
}

func (r *FrontmatterRequiredFieldsRule) emptyCategoriesIssue(filePath string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "categories list is empty",
		Explanation: "The `categories:` field is present but contains no items. " +
			"Hugo renders one sidebar section per category; an empty list leaves " +
			"the doc without a sidebar slot.",
		Fix: "Add at least one category, e.g. `categories: [explanation]`.",
		Line: 0,
	}
}

func (r *FrontmatterRequiredFieldsRule) malformedDateIssue(filePath, value string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityWarning,
		Rule:     r.Name(),
		Message:  "date does not match RFC 3339 / ISO 8601",
		Explanation: "`date` should be an RFC 3339 / ISO 8601 timestamp (e.g. " +
			"`2026-01-01T00:00:00Z`). Got `" + value + "`.",
		Fix:  "Update `date:` to an RFC 3339 / ISO 8601 timestamp.",
		Line: 0,
	}
}

func (r *FrontmatterRequiredFieldsRule) malformedLastmodIssue(filePath, value string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityWarning,
		Rule:     r.Name(),
		Message:  "lastmod does not match YYYY-MM-DD",
		Explanation: "`lastmod` should be a `YYYY-MM-DD` date (e.g. `2026-01-01`). Got `" + value + "`.",
		Fix:  "Update `lastmod:` to a `YYYY-MM-DD` date.",
		Line: 0,
	}
}

func (r *FrontmatterRequiredFieldsRule) malformedYAMLError(filePath string, err error) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "Frontmatter YAML could not be parsed",
		Explanation: "The YAML frontmatter could not be parsed: " + err.Error(),
		Fix:         "Fix the YAML syntax error reported above.",
		Line:        0,
	}
}
