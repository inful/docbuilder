package lint

import (
	"fmt"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"github.com/inful/mdfp"
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

const (
	frontmatterFingerprintHashKeyAliases = "aliases"
	frontmatterFingerprintHashKeyLastmod = "lastmod"
	frontmatterFingerprintHashKeyUID     = "uid"
)

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
		return nil, fmt.Errorf("read file: %w", err)
	}

	frontmatterBytes, bodyBytes, hadFrontmatter, _, splitErr := frontmatter.Split(data)
	if splitErr != nil {
		//nolint:nilerr // Split failures are reported as lint issues, not fatal errors.
		return []Issue{
			{
				FilePath: filePath,
				Severity: SeverityError,
				Rule:     r.Name(),
				Message:  splitErr.Error(),
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

	if !hadFrontmatter {
		return []Issue{
			{
				FilePath: filePath,
				Severity: SeverityError,
				Rule:     r.Name(),
				Message:  "Missing or invalid fingerprint in frontmatter",
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

	fields, parseErr := frontmatter.ParseYAML(frontmatterBytes)
	if parseErr != nil {
		return []Issue{
			{
				FilePath: filePath,
				Severity: SeverityError,
				Rule:     r.Name(),
				Message:  fmt.Sprintf("invalid YAML frontmatter: %v", parseErr),
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

	currentAny, ok := fields[mdfp.FingerprintField]
	if !ok {
		return []Issue{
			{
				FilePath: filePath,
				Severity: SeverityError,
				Rule:     r.Name(),
				Message:  "Missing or invalid fingerprint in frontmatter",
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

	currentFingerprint, ok := currentAny.(string)
	if !ok || strings.TrimSpace(currentFingerprint) == "" {
		return []Issue{
			{
				FilePath: filePath,
				Severity: SeverityError,
				Rule:     r.Name(),
				Message:  "Missing or invalid fingerprint in frontmatter",
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

	fieldsForHash := make(map[string]any, len(fields))
	for k, v := range fields {
		if k == mdfp.FingerprintField {
			continue
		}
		if k == frontmatterFingerprintHashKeyLastmod {
			continue
		}
		if k == frontmatterFingerprintHashKeyUID {
			continue
		}
		if k == frontmatterFingerprintHashKeyAliases {
			continue
		}
		fieldsForHash[k] = v
	}

	frontmatterForHash := ""
	if len(fieldsForHash) > 0 {
		serialized, serializeErr := frontmatter.SerializeYAML(fieldsForHash, frontmatter.Style{Newline: "\n"})
		if serializeErr != nil {
			return nil, fmt.Errorf("serialize frontmatter for fingerprint check: %w", serializeErr)
		}
		frontmatterForHash = strings.TrimSuffix(string(serialized), "\n")
	}

	expected := mdfp.CalculateFingerprintFromParts(frontmatterForHash, string(bodyBytes))
	if expected == currentFingerprint {
		return nil, nil
	}

	message := "Missing or invalid fingerprint in frontmatter"

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
