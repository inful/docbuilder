package lint

import (
	"net/url"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// WebSourcesRule reports an ERROR when the `web_sources` front matter
// field is not a list of http(s) URLs.
//
// The field asks ragabast to mirror external pages into the corpus.
// Ragabast reads a list of URLs: a single URL written as a string is
// not read, and the document is ingested while the mirror silently does
// not exist. The scheme of the entries matters too - ragabast fetches
// the page, and a page that is not http(s) is not a page of the web.
//
// The fix turns a lone string into a list of one. An entry that is not
// an http(s) URL is left to the author: the fixer does not guess a
// scheme and does not drop a page the author asked for.
type WebSourcesRule struct{}

const (
	// webSourcesNotAListMessage says that the field is not a list.
	webSourcesNotAListMessage = "The web sources must be a list"

	// webSourcesNotAURLMessage says that an entry is not a URL of the
	// web.
	webSourcesNotAURLMessage = "The web source must be an http(s) URL"
)

// Name returns the rule identifier.
func (r *WebSourcesRule) Name() string {
	return ruleWebSources
}

// AppliesTo returns true for documentation files.
func (r *WebSourcesRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// Check reads the frontmatter and reports the web sources that are not
// a list of http(s) URLs.
func (r *WebSourcesRule) Check(filePath string) ([]Issue, error) {
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
	raw, ok := fields["web_sources"]
	if !ok || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case string:
		// A lone URL: the fixer turns it into a list of one.
		return []Issue{r.notAListIssue(filePath, v)}, nil
	case []any:
		var issues []Issue
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				issues = append(issues, r.notAStringIssue(filePath))
				continue
			}
			if issue, bad := r.badURLIssue(filePath, s); bad {
				issues = append(issues, issue)
			}
		}
		return issues, nil
	case []string:
		var issues []Issue
		for _, s := range v {
			if issue, bad := r.badURLIssue(filePath, s); bad {
				issues = append(issues, issue)
			}
		}
		return issues, nil
	default:
		return []Issue{r.notAListIssue(filePath, "")}, nil
	}
}

// isWebSourceURL reports whether the value is an address of the web
// that ragabast can fetch: an http or https URL with a host.
func isWebSourceURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return false
	}
	return parsed.Host != ""
}

func (r *WebSourcesRule) notAListIssue(filePath, value string) Issue {
	detail := ""
	if value != "" {
		detail = " The field holds `" + value + "`."
	}
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  webSourcesNotAListMessage,
		Explanation: "The `web_sources` field asks ragabast to mirror external pages into the corpus, " +
			"and ragabast reads a list of URLs." + detail +
			" A field that is not a list is not read: the document is ingested and the mirror " +
			"silently does not exist.",
		Fix:  "Write the field as a list, or run `docbuilder lint --fix` to turn a lone URL into a list of one.",
		Line: 0,
	}
}

func (r *WebSourcesRule) notAStringIssue(filePath string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  webSourcesNotAURLMessage,
		Explanation: "The entries of the `web_sources` field are addresses of the web, and an entry " +
			"that is not a string is not one.",
		Fix:  "Write every entry of the field as an http or https URL.",
		Line: 0,
	}
}

func (r *WebSourcesRule) badURLIssue(filePath, value string) (Issue, bool) {
	if isWebSourceURL(value) {
		return Issue{}, false
	}
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  webSourcesNotAURLMessage,
		Explanation: "The entries of the `web_sources` field are addresses of the web: ragabast " +
			"fetches them, and only an http or https URL is a page of the web. `" + value + "` is not one.",
		Fix:  "Write the entry as an http or https URL, or remove it.",
		Line: 0,
	}, true
}
