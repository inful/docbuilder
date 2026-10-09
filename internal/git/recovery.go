package git

import (
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// gitCorruptionPatterns are lower-cased substrings of go-git's pack/idx/index
// decoder errors. They indicate that the on-disk git state in the workspace
// is unreadable (truncated pack, bad checksum, missing/corrupt index) and
// cannot be repaired in place — the recovery path must wipe and re-clone.
//
// Sourced from go-git v5.19.2:
//   - plumbing/format/index/decoder.go    (ErrInvalidChecksum)
//   - plumbing/format/idxfile/decoder.go  (ErrMalformedIdxFile, "checksum mismatch")
//   - plumbing/format/packfile/scanner.go (ErrMalformedPackFile, ErrEmptyPackfile,
//     ErrBadSignature, ErrUnsupportedVersion,
//     ErrInflatedSizeMismatch)
//   - storage/filesystem/object.go        ("packfile mismatch")
//   - storage/filesystem/object.go        (plumbing.ErrObjectNotFound — surfaces
//     as "object not found" when the .idx
//     is corrupt: the object is in the
//     pack but go-git can't locate it
//     without a valid index)
//
// "object not found" is intentionally listed even though it can in principle
// mean a legitimately-missing remote object. The self-heal (wipe + re-clone)
// is safe in both cases — if the remote truly doesn't have the object, the
// re-clone fails and the retry surfaces the real error; the worst case is a
// wasted re-clone for a very rare error shape. The benefit is that corrupt
// .idx files (which are otherwise indistinguishable from genuine missing
// objects at the error-message level) are recovered transparently.
var gitCorruptionPatterns = []string{
	"invalid checksum",
	"checksum mismatch",
	"malformed idx file",
	"malformed pack file",
	"empty packfile",
	"malformed pack file signature",
	"packfile mismatch",
	"unsupported packfile version",
	"inflated object exceeds declared size",
	"object not found",
}

// isCorruptionMessage reports whether the lower-cased error text matches any
// of the known go-git corruption signatures.
func isCorruptionMessage(lowerMsg string) bool {
	for _, p := range gitCorruptionPatterns {
		if strings.Contains(lowerMsg, p) {
			return true
		}
	}
	return false
}

// isCorruptionError reports whether err indicates corrupt on-disk git state
// that justifies a wipe + re-clone. Checks the classified-error context first
// (set by ClassifyGitError) and falls back to a substring match so that
// unclassified errors — or errors from go-git paths that bypass our
// classifier — are still recoverable.
//
// The "object not found" pattern is included here even though it can
// theoretically come from a legitimately-missing remote object; the trade-off
// is documented on gitCorruptionPatterns above. Callers in the update path
// can rely on this: if updateExistingRepo returns an error matching the
// corruption patterns, the local pack/idx state is the most likely cause,
// and a wipe + re-clone is the cheapest correct response.
func isCorruptionError(err error) bool {
	if err == nil {
		return false
	}
	if ce, ok := errors.AsClassified(err); ok {
		if v, ok := ce.Context()["corruption"]; ok {
			if b, ok := v.(bool); ok && b {
				return true
			}
		}
	}
	msg := strings.ToLower(err.Error())
	return isCorruptionMessage(msg)
}
