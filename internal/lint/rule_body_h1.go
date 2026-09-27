package lint

import (
	"bufio"
	"bytes"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// BodyH1Rule flags top-level H1 headings (`# `) that appear at the
// start of a markdown body. The page's H1 is rendered from the
// `title` frontmatter field, so a body H1 duplicates the rendered
// heading and breaks the section-page layout.
//
// Reported as WARNING severity per the issue discussion: surfaces in
// `lint_docs` output without blocking the audit test.
type BodyH1Rule struct{}

// Name returns the rule identifier.
func (r *BodyH1Rule) Name() string {
	return ruleBodyH1
}

// AppliesTo returns true for documentation files.
func (r *BodyH1Rule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// Check scans the file's body for a leading `# ` line.
func (r *BodyH1Rule) Check(filePath string) ([]Issue, error) {
	// #nosec G304 -- filePath comes from controlled doc discovery/lint walk.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	_, body, _, _, splitErr := frontmatter.Split(data)
	if splitErr != nil {
		//nolint:nilerr // split errors are reported as lint issues by other rules.
		return nil, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		// Skip leading blank lines (the rule allows them per the issue).
		if strings.TrimSpace(line) == "" {
			continue
		}
		// First non-blank content line: if it starts with `# ` (H1), flag it.
		if bytes.HasPrefix([]byte(line), []byte("# ")) {
			return []Issue{r.bodyH1Issue(filePath)}, nil
		}
		return nil, nil
	}
	return nil, nil
}

func (r *BodyH1Rule) bodyH1Issue(filePath string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityWarning,
		Rule:     r.Name(),
		Message:  "Body begins with a top-level H1",
		Explanation: "The page's H1 is rendered from the `title` frontmatter field. " +
			"A body H1 duplicates the rendered heading and confuses the reader " +
			"(which title is the real one?). Start the body with H2 or deeper.",
		Fix: "Remove the `# Title` line from the body, or change it to `## Title`.",
		Line: 1,
	}
}
