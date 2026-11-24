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

// PatchFingerprint rewrites the fingerprint (and the lastmod) in the
// raw frontmatter bytes.
//
// The frontmatter is not parsed and written again: the comments of the
// file and the order of its keys survive the fix, and the diff of a
// fingerprint fix is the fingerprint (ADR-014).
func PatchFingerprint(fm []byte, fingerprint, lastmod string) []byte {
	if fingerprint != "" {
		fm = patchScalar(fm, mdfp.FingerprintField, fingerprint)
	}
	if lastmod != "" {
		fm = patchScalar(fm, fingerprintHashKeyLastmod, lastmod)
	}
	return fm
}

// patchScalar rewrites the value of a top-level key of the raw
// frontmatter. A key that is not there is added at the end.
//
// The line endings of the file survive: a file with CRLF keeps them.
func patchScalar(fm []byte, key, value string) []byte {
	text := string(fm)
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}

	lines := strings.Split(text, "\n")

	// The empty lines at the end are the end of the frontmatter.
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	for i := range end {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, rest, ok := strings.Cut(trimmed, ":")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		newValue := value
		if strings.HasPrefix(strings.TrimSpace(rest), "\"") {
			newValue = "\"" + value + "\""
		}
		lineEnding := ""
		if strings.HasSuffix(line, "\r") {
			lineEnding = "\r"
		}
		lines[i] = indent + key + ": " + newValue + lineEnding
		return []byte(strings.Join(lines, "\n"))
	}

	// The key is not there: it is added at the end of the frontmatter.
	kept := append([]string{}, lines[:end]...)
	kept = append(kept, key+": "+value)
	return []byte(strings.Join(kept, "\n") + newline)
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
