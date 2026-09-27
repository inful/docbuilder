package lint

import (
	"path/filepath"
	"regexp"
)

// SequencePrefixFilenameRule verifies that files in sequence-numbered
// directories follow the documented prefix convention. Currently
// only `adr` is registered, mapping to `adr-`; the rule's SequenceDirs
// field allows extending to other sequence types
// (runbook-, tutorial-, …) without code changes.
//
// The required pattern for a directory mapped to prefix `<prefix>-` is
// `^<prefix>-\d{3}-.+\.md$` — a fixed-width 3-digit zero-padded
// sequence number, a hyphen, a slug, and the `.md` extension.
type SequencePrefixFilenameRule struct {
	// SequenceDirs maps directory basename → required prefix.
	// Defaults to {"adr": "adr-"}.
	SequenceDirs map[string]string
}

// Name returns the rule identifier.
func (r *SequencePrefixFilenameRule) Name() string {
	return ruleSequencePrefixFilename
}

// AppliesTo returns true for any file whose directory is a configured
// sequence directory.
func (r *SequencePrefixFilenameRule) AppliesTo(filePath string) bool {
	dir := filepath.Base(filepath.Dir(filePath))
	_, ok := r.sequenceDirs()[dir]
	return ok
}

// Check returns an ERROR if the filename doesn't match the required pattern.
func (r *SequencePrefixFilenameRule) Check(filePath string) ([]Issue, error) {
	dir := filepath.Base(filepath.Dir(filePath))
	prefix, ok := r.sequenceDirs()[dir]
	if !ok {
		return nil, nil
	}
	filename := filepath.Base(filePath)
	pattern := "^" + regexp.QuoteMeta(prefix) + `\d{3}-.+\.md$`
	matched, err := regexp.MatchString(pattern, filename)
	if err != nil {
		return nil, err
	}
	if matched {
		return nil, nil
	}
	return []Issue{r.badSequenceIssue(filePath, prefix)}, nil
}

func (r *SequencePrefixFilenameRule) badSequenceIssue(filePath, prefix string) Issue {
	return Issue{
		FilePath: filePath,
		Severity: SeverityError,
		Rule:     r.Name(),
		Message:  "Filename does not match sequence pattern",
		Explanation: "Files in this directory must use the sequence prefix `" + prefix + "NNN-slug.md` " +
			"with a 3-digit zero-padded sequence number, e.g. `" + prefix + "001-foo.md`. " +
			"Stable sorting by filename then gives chronological reading order in listings.",
		Fix:  "Rename the file to use the `" + prefix + "NNN-slug.md` pattern with a 3-digit zero-padded number.",
		Line: 0,
	}
}

func (r *SequencePrefixFilenameRule) sequenceDirs() map[string]string {
	if r.SequenceDirs != nil {
		return r.SequenceDirs
	}
	return map[string]string{"adr": "adr-"}
}
