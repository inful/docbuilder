package lint

import (
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// CrossModeCategoryRule reports an ERROR when a doc's `categories:`
// field lists two distinct doc-mode categories that aren't in an
// allowed parent/child relationship.
//
// Hugo renders the doc in each sidebar section corresponding to a
// category. A doc in both `how-to` and `reference` sections is a sign
// that the author wasn't sure what kind of doc it is. The rule forces
// the author to pick one — and if the doc is genuinely cross-cutting,
// they can split it into two pages with cross-links.
//
// The set of allowed parent/child pairs is configured via AllowedPairs.
// When empty (the default), no parent/child pairs are allowed.
type CrossModeCategoryRule struct {
	// AllowedPairs maps a child category → set of allowed parent categories.
	// For example, if `reference` is allowed as a child of `tutorials`,
	// set AllowedPairs["reference"] = map[string]bool{"tutorials": true}.
	AllowedPairs map[string]map[string]bool
}

// Name returns the rule identifier.
func (r *CrossModeCategoryRule) Name() string {
	return ruleCrossModeCategory
}

// AppliesTo returns true for documentation files.
func (r *CrossModeCategoryRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// Check reads frontmatter and flags any pair of distinct categories that
// isn't in AllowedPairs.
func (r *CrossModeCategoryRule) Check(filePath string) ([]Issue, error) {
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
	catsAny, ok := fields["categories"]
	if !ok {
		return nil, nil
	}
	cats := listStrings(catsAny)
	if len(cats) < 2 {
		return nil, nil
	}

	// Deduplicate (categories may repeat in frontmatter).
	unique := uniqueStrings(cats)
	if len(unique) < 2 {
		return nil, nil
	}

	var issues []Issue
	for i, a := range unique {
		for j := i + 1; j < len(unique); j++ {
			b := unique[j]
			if !r.isPairAllowed(a, b) {
				issues = append(issues, r.badPairIssue(filePath, a, b))
			}
		}
	}
	return issues, nil
}

func (r *CrossModeCategoryRule) isPairAllowed(a, b string) bool {
	if r.AllowedPairs == nil {
		return false
	}
	if r.AllowedPairs[a] != nil && r.AllowedPairs[a][b] {
		return true
	}
	if r.AllowedPairs[b] != nil && r.AllowedPairs[b][a] {
		return true
	}
	return false
}

func (r *CrossModeCategoryRule) badPairIssue(filePath, a, b string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "Doc has two distinct doc-mode categories",
		Explanation: "Categories `" + a + "` and `" + b + "` are distinct doc modes. " +
			"Hugo renders the doc in each sidebar section, which produces a confusing entry " +
			"that mixes doc kinds. Pick one — if the doc is genuinely cross-cutting, " +
			"split it into two pages with cross-links.",
		Fix:  "Reduce `categories:` to a single doc mode, or split the doc into two pages.",
		Line: 0,
	}
}

// uniqueStrings returns a deduplicated, original-order slice of v.
func uniqueStrings(v []string) []string {
	seen := make(map[string]bool, len(v))
	out := make([]string, 0, len(v))
	for _, s := range v {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
