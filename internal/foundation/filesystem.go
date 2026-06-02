package foundation

import (
	"os"
	"path/filepath"
	"sync"
)

// IsCaseSensitiveFilesystem reports whether the given directory is on a
// case-sensitive filesystem. It probes by creating two files in the
// directory whose names differ only in case, then checking whether they
// coexist (case-sensitive) or whether the second write overwrote the first
// (case-insensitive, e.g. macOS HFS+/APFS, Windows NTFS).
//
// The result is cached per directory because the answer cannot change
// during the test process lifetime.
//
// Tests that rely on case-only filename differences to assert behaviour
// (e.g. case-collision detection, rename-overwrite protection) should
// skip themselves on case-insensitive filesystems. Such tests cannot
// faithfully simulate the case-different names on a filesystem that
// silently folds them.
func IsCaseSensitiveFilesystem(dir string) bool {
	probeOnce.Do(func() {
		probeResult = probeIsCaseSensitive(probeDir)
	})
	if dir == "" {
		return probeResult
	}
	return probeIsCaseSensitive(dir)
}

var (
	probeOnce   sync.Once
	probeResult bool
	probeDir    = os.TempDir()
)

func probeIsCaseSensitive(dir string) bool {
	// Use a unique subdir to avoid races with other tests/probes.
	base, err := os.MkdirTemp(dir, "caseprobe-*")
	if err != nil {
		// If we can't even create a tempdir we cannot decide; assume
		// case-insensitive so we err on the side of skipping tests
		// that need a strict filesystem.
		return false
	}
	defer os.RemoveAll(base)

	lower := filepath.Join(base, "lowercase")
	upper := filepath.Join(base, "LowerCase")

	if err := os.WriteFile(lower, []byte("a"), 0o600); err != nil {
		return false
	}
	defer os.Remove(lower)

	if err := os.WriteFile(upper, []byte("b"), 0o600); err != nil {
		return false
	}
	defer os.Remove(upper)

	// Read the directory listing: on a case-sensitive FS, both names
	// appear. On a case-insensitive FS, the second write overwrites
	// the first and only one name appears in the listing. (Note:
	// os.Stat on either name returns success on case-insensitive FS
	// because the FS aliases both names to the same inode, so the
	// directory listing is the authoritative check.)
	entries, err := os.ReadDir(base)
	if err != nil {
		return false
	}
	names := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		names[e.Name()] = struct{}{}
	}
	_, hasLower := names["lowercase"]
	_, hasUpper := names["LowerCase"]
	return hasLower && hasUpper
}
