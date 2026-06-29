// Package hashutil centralizes small content-hashing helpers used across
// the build pipeline. The first one, PathsHash, lived as a 5-line
// sha256-with-NUL-separator block in two places (build/delta/delta_analyzer.go
// and build/delta/manager.go). The plan's M8 called them out as
// "byte-for-byte identical"; in practice the two callers had diverged
// slightly (one sorted, the other didn't; one short-circuited empty
// paths, the other did not). The version below is the canonical
// semantics: empty -> "", sorted NUL-separated SHA-256 -> hex.
package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// separator is NUL because the legal Unix/Linux/macOS path byte never
// contains it. Slashes/commas/spaces are not safe separators (a path
// containing the separator could collide with a multi-path input).
const separator = byte(0)

// PathsHash returns a stable hash of a set of paths, suitable for
// "did the set of files change?" comparisons across builds. It hashes
// the sorted concatenation with a NUL separator between paths.
//
// Sorting matters: filepath.Walk returns paths in filesystem-dependent
// order, so without a stable sort the same set of files on the same
// tree can produce different hashes across runs. Sorting here is the
// single canonical place where the order is pinned; callers pass the
// slice they have and never need to sort.
//
// Returns "" when paths is empty; callers persist that as "no doc
// files for this repo", which downstream treats as an explicit
// "nothing to check" rather than a missing-value sentinel.
func PathsHash(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	// Copy so we don't mutate the caller's slice. Many callers pass
	// slices they iterate immediately afterwards; sorting in place
	// would surprise them.
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)

	h := sha256.New()
	for _, p := range sorted {
		h.Write([]byte(p))
		h.Write([]byte{separator})
	}
	return hex.EncodeToString(h.Sum(nil))
}
