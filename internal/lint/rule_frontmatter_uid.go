package lint

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type FrontmatterUIDRule struct{}

const (
	frontmatterUIDRuleName = "frontmatter-uid"
	missingUIDMessage      = "Missing uid in frontmatter"
	invalidUIDMessage      = "Invalid uid format in frontmatter"
	missingUIDaliasMessage = "Missing uid-based alias in frontmatter"
	indexFilename          = "_index.md"
)

func (r *FrontmatterUIDRule) Name() string {
	return frontmatterUIDRuleName
}

func (r *FrontmatterUIDRule) AppliesTo(filePath string) bool {
	// Skip generated index files - they don't need UIDs
	if strings.HasSuffix(filePath, "/"+indexFilename) || strings.HasSuffix(filePath, "\\"+indexFilename) || filePath == indexFilename {
		return false
	}
	return IsDocFile(filePath)
}

func (r *FrontmatterUIDRule) Check(filePath string) ([]Issue, error) {
	// #nosec G304 -- filePath is derived from the current lint target.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	fm, ok := extractFrontmatter(string(data))
	if !ok {
		return []Issue{r.missingIssue(filePath)}, nil
	}

	var obj map[string]any
	if err := yaml.Unmarshal([]byte(fm), &obj); err != nil {
		// If frontmatter exists but isn't valid YAML, other rules may report it,
		// but uid can't be validated either.
		return []Issue{r.missingIssue(filePath)}, nil //nolint:nilerr // reported as lint issue, not a hard error
	}

	uidAny, hasUID := obj["uid"]
	if !hasUID {
		return []Issue{r.missingIssue(filePath)}, nil
	}

	uidStr, ok := uidAny.(string)
	if !ok {
		return []Issue{r.invalidIssue(filePath, fmt.Sprintf("uid must be a string, got %T", uidAny))}, nil
	}

	uidStr = strings.TrimSpace(uidStr)
	if uidStr == "" {
		return []Issue{r.invalidIssue(filePath, "uid is empty")}, nil
	}

	if _, err := uuid.Parse(uidStr); err != nil {
		return []Issue{r.invalidIssue(filePath, "uid must be a valid GUID/UUID")}, nil //nolint:nilerr // reported as lint issue, not a hard error
	}

	// Check for uid-based alias
	expectedAlias := "/_uid/" + uidStr + "/"
	aliasesAny, hasAliases := obj["aliases"]
	if !hasAliases {
		return []Issue{r.missingAliasIssue(filePath, uidStr)}, nil
	}

	// aliases can be a string or array
	var aliasesList []string
	switch v := aliasesAny.(type) {
	case string:
		aliasesList = []string{v}
	case []any:
		for _, item := range v {
			if str, ok := item.(string); ok {
				aliasesList = append(aliasesList, str)
			}
		}
	case []string:
		aliasesList = v
	default:
		// Invalid aliases format, but let it pass to avoid false positives
		return nil, nil
	}

	// Check if the expected alias is present
	for _, alias := range aliasesList {
		if strings.TrimSpace(alias) == expectedAlias {
			return nil, nil
		}
	}

	return []Issue{r.missingAliasIssue(filePath, uidStr)}, nil
}

func (r *FrontmatterUIDRule) missingIssue(filePath string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     frontmatterUIDRuleName,
		Message:  missingUIDMessage,
		Explanation: strings.TrimSpace(strings.Join([]string{
			"This document is expected to carry a stable unique identifier (uid) in its YAML frontmatter.",
			"The uid must be generated once and must never be changed.",
			"It should be a GUID/UUID string.",
		}, "\n")),
		Fix:  "Run: docbuilder lint --fix (adds missing frontmatter uid fields)",
		Line: 0,
	}
}

func (r *FrontmatterUIDRule) invalidIssue(filePath, detail string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     frontmatterUIDRuleName,
		Message:  invalidUIDMessage,
		Explanation: strings.TrimSpace(strings.Join([]string{
			"This document has a uid in YAML frontmatter, but it is not a valid GUID/UUID string.",
			"The uid must be stable and must never be changed once correct.",
			"",
			"Details: " + detail,
		}, "\n")),
		Fix:  "Manually update the uid to a valid GUID/UUID (do not change it once set).",
		Line: 0,
	}
}

func (r *FrontmatterUIDRule) missingAliasIssue(filePath, uid string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     frontmatterUIDRuleName,
		Message:  missingUIDaliasMessage,
		Explanation: strings.TrimSpace(strings.Join([]string{
			"This document has a valid uid but is missing the corresponding alias in frontmatter.",
			"Documents should include a stable /_uid/<uid>/ alias for durable external references.",
			"",
			"Expected alias: /_uid/" + uid + "/",
		}, "\n")),
		Fix:  "Run: docbuilder lint --fix (adds missing uid-based aliases)",
		Line: 0,
	}
}

// extractFrontmatter returns the YAML frontmatter (without delimiters) if present.
func extractFrontmatter(content string) (string, bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return "", false
	}
	frontmatter := content[4 : endIdx+4]
	return frontmatter, true
}
