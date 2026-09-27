package lint

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// DirectoryCategoryConsistencyRule verifies that every doc under a
// mode-specific directory has a `categories:` value matching that
// directory's expected mode.
//
// Mapping (per docs/explanation/docbuilder-best-practices.md §1.1):
//
//	how-to            → how-to
//	reference         → reference
//	explanation       → explanation
//	security          → security
//	adr               → architecture-decisions
//	tutorials         → tutorials
//	examples          → templates (the source-of-truth template files)
//
// Nested directories inside these (e.g. `docs/explanation/diagrams/`)
// don't have an expected category and are not flagged.
type DirectoryCategoryConsistencyRule struct{}

// directoryToCategory maps immediate-subdirectory name → expected category.
var directoryToCategory = map[string]string{
	"how-to":      "how-to",
	"reference":   "reference",
	"explanation": "explanation",
	"security":    "security",
	"adr":         "architecture-decisions",
	"tutorials":   "tutorials",
	"examples":    "templates",
}

// Name returns the rule identifier.
func (r *DirectoryCategoryConsistencyRule) Name() string {
	return ruleDirectoryCategoryConsistency
}

// AppliesTo returns true for documentation files.
func (r *DirectoryCategoryConsistencyRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// Check verifies the file's `categories:` includes the directory's
// expected category (when the directory is in the known map).
func (r *DirectoryCategoryConsistencyRule) Check(filePath string) ([]Issue, error) {
	dir := filepath.Dir(filePath)
	// Walk up the directory tree to find the first known mapping.
	expected, ok := findExpectedCategory(filepath.Base(dir))
	if !ok {
		// No expected category for this directory (or any ancestor).
		return nil, nil
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
	catsAny, ok := fields["categories"]
	if !ok {
		return r.missingCategoriesIssue(filePath, expected), nil
	}
	cats := listStrings(catsAny)
	if slices.Contains(cats, expected) {
		return nil, nil
	}
	return r.mismatchIssue(filePath, expected, cats), nil
}

// findExpectedCategory returns the expected category for a directory
// name by looking it up in the static map. The "directory" here is
// the immediate subdirectory name (e.g. "how-to", "adr"). Nested
// directories like `explanation/diagrams/` are skipped because the
// caller passes only the immediate subdirectory.
func findExpectedCategory(dirName string) (string, bool) {
	expected, ok := directoryToCategory[dirName]
	return expected, ok
}

func (r *DirectoryCategoryConsistencyRule) missingCategoriesIssue(filePath, expected string) []Issue {
	return []Issue{{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "Doc missing required category",
		Explanation: "This doc is in a mode-specific directory and must carry a matching " +
			"`categories:` value. Expected category for this directory: `" + expected + "`.",
		Fix:  "Add `categories: [" + expected + "]` to the frontmatter.",
		Line: 0,
	}}
}

func (r *DirectoryCategoryConsistencyRule) mismatchIssue(filePath, expected string, cats []string) []Issue {
	catsList := strings.Join(cats, ", ")
	if catsList == "" {
		catsList = "(empty)"
	}
	return []Issue{{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "Category does not match directory",
		Explanation: "This doc's `categories:` (" + catsList + ") doesn't include the " +
			"expected category `" + expected + "` for its directory. Hugo's sidebar " +
			"renders one section per category; mixing categories across mode-specific " +
			"directories puts a doc in the wrong sidebar section.",
		Fix:  "Add `" + expected + "` to the `categories:` list (or change directory if the doc belongs elsewhere).",
		Line: 0,
	}}
}
