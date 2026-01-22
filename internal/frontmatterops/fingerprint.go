package frontmatterops

import (
	"errors"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"github.com/inful/mdfp"
)

const (
	fingerprintHashKeyAliases = "aliases"
	fingerprintHashKeyLastmod = "lastmod"
	fingerprintHashKeyUID     = "uid"
)

// ComputeFingerprint computes the canonical content fingerprint for a document.
//
// It matches the current DocBuilder canonicalization rules:
//   - excludes: fingerprint, lastmod, uid, aliases
//   - serializes YAML with LF newlines
//   - trims a single trailing newline from the serialized YAML before hashing
func ComputeFingerprint(fields map[string]any, body []byte) (string, error) {
	if fields == nil {
		return "", errors.New("fields map is nil")
	}

	fieldsForHash := make(map[string]any, len(fields))
	for k, v := range fields {
		if k == mdfp.FingerprintField {
			continue
		}
		if k == fingerprintHashKeyLastmod {
			continue
		}
		if k == fingerprintHashKeyUID {
			continue
		}
		if k == fingerprintHashKeyAliases {
			continue
		}
		fieldsForHash[k] = v
	}

	frontmatterForHash := ""
	if len(fieldsForHash) > 0 {
		serialized, err := frontmatter.SerializeYAML(fieldsForHash, frontmatter.Style{Newline: "\n"})
		if err != nil {
			return "", err
		}
		frontmatterForHash = trimSingleTrailingNewline(string(serialized))
	}

	return mdfp.CalculateFingerprintFromParts(frontmatterForHash, string(body)), nil
}

// UpsertFingerprintAndMaybeLastmod computes and upserts the canonical fingerprint.
//
// If the fingerprint changes (and is non-empty), it also updates lastmod to the provided
// time in UTC, formatted as "2006-01-02" (matching the current lint fixer behavior).
func UpsertFingerprintAndMaybeLastmod(fields map[string]any, body []byte, now time.Time) (fingerprint string, changed bool, err error) {
	if fields == nil {
		return "", false, errors.New("fields map is nil")
	}

	oldFP, _ := fields[mdfp.FingerprintField].(string)

	fingerprint, err = ComputeFingerprint(fields, body)
	if err != nil {
		return "", false, err
	}

	if existing, ok := fields[mdfp.FingerprintField].(string); !ok || existing != fingerprint {
		fields[mdfp.FingerprintField] = fingerprint
		changed = true
	}

	// ADR-011: If fingerprint changes, update lastmod (YYYY-MM-DD, UTC).
	if fingerprint != "" && strings.TrimSpace(fingerprint) != strings.TrimSpace(oldFP) {
		fields[fingerprintHashKeyLastmod] = now.UTC().Format("2006-01-02")
		changed = true
	}

	return fingerprint, changed, nil
}

func trimSingleTrailingNewline(s string) string {
	if before, ok := strings.CutSuffix(s, "\r\n"); ok {
		return before
	}
	if before, ok := strings.CutSuffix(s, "\n"); ok {
		return before
	}
	return s
}
