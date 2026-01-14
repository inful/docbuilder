package lint

import (
	"os"
	"strings"

	"github.com/inful/mdfp"

	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// FrontmatterFingerprintRule verifies that markdown files have a valid content fingerprint
// stored in YAML frontmatter.
//
// It uses github.com/inful/mdfp to:
//   - detect missing fingerprints
//   - detect mismatched fingerprints (content changed without updating the fingerprint)
//
// The fixer can regenerate fingerprints for any issues emitted by this rule.
type FrontmatterFingerprintRule struct{}

const frontmatterFingerprintRuleName = "frontmatter-fingerprint"

func (r *FrontmatterFingerprintRule) Name() string {
	return frontmatterFingerprintRuleName
}

func (r *FrontmatterFingerprintRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

func (r *FrontmatterFingerprintRule) Check(filePath string) ([]Issue, error) {
	// #nosec G304 -- filePath comes from controlled doc discovery/lint walk.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
			"failed to read file for fingerprint check").
			WithContext("file", filePath).
			Build()
	}

	ok, verifyErr := mdfp.VerifyFingerprint(string(data))
	if ok {
		return nil, nil
	}

	// mdfp uses errors to signal both missing and mismatched fingerprints.
	// Treat all verification failures as a fixable error.
	message := "Missing or invalid fingerprint in frontmatter"
	if verifyErr != nil {
		message = verifyErr.Error()
	}

	return []Issue{
		{
			FilePath: filePath,
			Severity: SeverityError,
			Rule:     r.Name(),
			Message:  message,
			Explanation: strings.TrimSpace(strings.Join([]string{
				"This document is expected to carry a content fingerprint in its YAML frontmatter.",
				"DocBuilder uses these fingerprints to detect content changes reliably.",
				"",
				"This check is powered by github.com/inful/mdfp.",
			}, "\n")),
			Fix: "Run: docbuilder lint --fix (regenerates frontmatter fingerprints)",
		},
	}, nil
}
