package lint

import (
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// TagCountRule warns when a doc's `tags:` list exceeds the configured
// threshold (default 10). Tags create orthogonal cross-sections and
// should stay curated; past ~10 entries the list becomes noise rather
// than signal.
//
// WARNING severity (not ERROR) per the issue: recommendation, not a
// blocking rule.
type TagCountRule struct {
	// Threshold is the maximum number of tags before the rule reports.
	// Defaults to 10 if zero or negative.
	Threshold int
}

// Name returns the rule identifier.
func (r *TagCountRule) Name() string {
	return ruleTagCount
}

// AppliesTo returns true for documentation files.
func (r *TagCountRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// Check reads the parsed frontmatter and reports a WARNING if tags > threshold.
func (r *TagCountRule) Check(filePath string) ([]Issue, error) {
	threshold := r.Threshold
	if threshold <= 0 {
		threshold = 10
	}

	// #nosec G304 -- filePath is from controlled lint walk.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	fmBytes, _, had, _, splitErr := frontmatter.Split(data)
	if splitErr != nil || !had {
		//nolint:nilerr // reported as lint issue, not a hard error
		return nil, nil
	}
	fields, parseErr := frontmatter.ParseYAML(fmBytes)
	if parseErr != nil {
		//nolint:nilerr // reported as lint issue, not a hard error
		return nil, nil
	}
	tagsAny, ok := fields["tags"]
	if !ok {
		return nil, nil
	}
	count := countListAny(tagsAny)
	if count > threshold {
		return []Issue{r.tooManyTags(filePath, count, threshold)}, nil
	}
	return nil, nil
}

func (r *TagCountRule) tooManyTags(filePath string, count, threshold int) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityWarning,
		Rule:     r.Name(),
		Message:  "Too many tags",
		Explanation: "This doc has " + itoa(count) + " tags (threshold: " + itoa(threshold) + "). " +
			"Tags create orthogonal cross-sections; past ~10 entries the list stops being curated " +
			"and becomes noise. Consider consolidating related tags.",
		Fix:  "Remove redundant tags or split the doc into more focused pages.",
		Line: 0,
	}
}

// countListAny returns the number of items in a YAML list value.
// Accepts []any (parsed from frontmatter) or []string.
func countListAny(v any) int {
	switch x := v.(type) {
	case []any:
		return len(x)
	case []string:
		return len(x)
	}
	return 0
}
