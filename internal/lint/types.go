package lint

import "path/filepath"

const (
	docExtensionMarkdown     = ".md"
	docExtensionMarkdownLong = ".markdown"
)

// Severity indicates the importance level of a linting issue.
type Severity int

const (
	// SeverityInfo indicates informational messages (e.g., whitelisted patterns).
	SeverityInfo Severity = iota
	// SeverityWarning indicates issues that should be fixed but don't block builds.
	SeverityWarning
	// SeverityError indicates issues that will prevent Hugo builds from succeeding.
	SeverityError
)

// String returns the human-readable severity name.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Issue represents a single linting problem found in a file.
type Issue struct {
	FilePath    string   // Absolute or relative path to the file
	Severity    Severity // Issue severity level
	Rule        string   // Rule identifier (e.g., "filename-lowercase")
	Message     string   // Brief description of the issue
	Explanation string   // Detailed explanation with context
	Fix         string   // Suggested fix or command to resolve
	Line        int      // Line number (0 if file-level issue)
}

// Result contains all issues found during linting.
type Result struct {
	Issues     []Issue
	FilesTotal int // Total files scanned
}

// HasErrors returns true if any error-level issues exist.
func (r *Result) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any warning-level issues exist.
func (r *Result) HasWarnings() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error-level issues.
func (r *Result) ErrorCount() int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning-level issues.
func (r *Result) WarningCount() int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

// Rule defines a linting rule that can be applied to files.
type Rule interface {
	// Name returns the unique identifier for this rule.
	Name() string

	// Check validates a file and returns any issues found.
	Check(filePath string) ([]Issue, error)

	// AppliesTo returns true if this rule should be checked for the given file.
	AppliesTo(filePath string) bool
}

// DefaultExcludes is the cross-forge union of git-forge-conventional files
// that should be exempted from lint and schema checks. It covers GitHub,
// GitLab, and Forgejo conventions plus the nested-glob entries for the
// per-forge template directories.
//
// Entries are glob-style path patterns matched against the file's path
// relative to the repo root. A file matches if any pattern matches.
var DefaultExcludes = []string{
	// README — universal across GitHub, GitLab, Forgejo
	"README.md",
	"README",
	"README.rst",
	"README.txt",
	"README.adoc",

	// Contributing guidelines
	"CONTRIBUTING.md",
	"CONTRIBUTING",

	// Security policy — GitHub Security tab + GitLab auto-detected
	"SECURITY.md",
	"SECURITY",

	// Code of conduct — GitHub community health file
	"CODE_OF_CONDUCT.md",
	"CODE_OF_CONDUCT",

	// Support resources — GitHub community health file
	"SUPPORT.md",

	// Changelog — linked from Releases on GitHub/GitLab
	"CHANGELOG.md",
	"CHANGELOG",

	// License — auto-detected by every forge
	"LICENSE",
	"LICENSE.md",
	"LICENSE.txt",
	"LICENSE.rst",
	"COPYING",
	"COPYING.md",

	// Authors / credits
	"AUTHORS",
	"AUTHORS.md",
	"CONTRIBUTORS",
	"CONTRIBUTORS.md",

	// Legal notices
	"NOTICE",
	"NOTICE.md",

	// GitHub template directories
	".github/ISSUE_TEMPLATE/**/*.md",
	".github/PULL_REQUEST_TEMPLATE.md",

	// GitLab template directories
	".gitlab/issue_templates/*.md",
	".gitlab/merge_request_templates/*.md",

	// Forgejo / Gitea template directories
	".gitea/ISSUE_TEMPLATE/**/*.md",
	".gitea/PULL_REQUEST_TEMPLATE.md",
}

// Config contains configuration for the linter.
type Config struct {
	// Quiet suppresses warnings, only showing errors.
	Quiet bool

	// Format specifies output format (text, json).
	Format string

	// Fix enables automatic fixing of issues where possible.
	Fix bool

	// DryRun shows what would be fixed without applying changes.
	DryRun bool

	// Yes automatically confirms fixes without prompting.
	Yes bool

	// Excludes is the list of glob patterns identifying files that
	// should be skipped from lint and schema enforcement. Patterns are
	// matched against the file's path relative to the repo root using
	// path.Match semantics, with ** supported as a recursive wildcard.
	//
	// When nil, DefaultExcludes applies (the cross-forge union of
	// git-forge-conventional files). When an explicit empty slice is
	// supplied, no files are excluded.
	Excludes []string
}

// IsDocFile returns true if the file is a documentation file.
func IsDocFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == docExtensionMarkdown || ext == docExtensionMarkdownLong
}

// IsAssetFile returns true if the file is an image asset.
func IsAssetFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".svg"
}
