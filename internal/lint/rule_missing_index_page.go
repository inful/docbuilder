package lint

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// MissingIndexPageRule reports a WARNING when a directory has at least
// `minChildren` immediate `.md` children and no `_index.md` landing page.
//
// Hugo auto-generates a flat listing of children when there's no
// `_index.md` in a directory. For directories with many children that's
// a wall of links; a curated `_index.md` (one paragraph + 3-7 most
// important docs hand-picked) is the right answer for anything with
// more than two children.
//
// Flat-collection directories (adr/, examples/) are skipped — Hugo's
// auto-generated listing is the right answer there.
type MissingIndexPageRule struct {
	// MinChildren is the minimum number of immediate .md children
	// required before the rule reports. Defaults to 3 if zero.
	MinChildren int
	// FlatCollections is the set of directory basenames that are flat
	// collections and should be skipped. Defaults to {"adr", "examples"}.
	FlatCollections map[string]bool
}

// Name returns the rule identifier.
func (r *MissingIndexPageRule) Name() string {
	return ruleMissingIndexPage
}

// AppliesTo returns true for documentation files. The rule's actual
// work is done at the directory level via CheckDirectory; this method
// exists to satisfy the Rule interface.
func (r *MissingIndexPageRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath)
}

// Check is a no-op for the per-file lint loop. The Linter invokes
// CheckDirectory separately after the per-file pass.
func (r *MissingIndexPageRule) Check(filePath string) ([]Issue, error) {
	return nil, nil
}

// CheckDirectory walks the given directory and reports one issue per
// subdirectory with ≥ minChildren .md children and no _index.md.
func (r *MissingIndexPageRule) CheckDirectory(rootPath string) ([]Issue, error) {
	min := r.MinChildren
	if min <= 0 {
		min = 3
	}
	flat := r.FlatCollections
	if flat == nil {
		flat = map[string]bool{"adr": true, "examples": true}
	}

	var issues []Issue
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		// Skip the root itself
		if path == rootPath {
			return nil
		}
		// Skip hidden directories
		if strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}
		// Skip flat-collection directories
		if flat[d.Name()] {
			return fs.SkipDir
		}
		// Count immediate .md children; check for _index.md
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		hasIndex := false
		mdCount := 0
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if name == indexFilename {
				hasIndex = true
				continue
			}
			if IsDocFile(name) {
				mdCount++
			}
		}
		if mdCount >= min && !hasIndex {
			issues = append(issues, r.missingIndexIssue(path, mdCount))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return issues, nil
}

func (r *MissingIndexPageRule) missingIndexIssue(dirPath string, count int) Issue {
	return Issue{
		FilePath: filepath.Join(dirPath, indexFilename),
		Severity: SeverityWarning,
		Rule:     r.Name(),
		Message:  "Directory missing _index.md landing page",
		Explanation: "This directory has " + itoa(count) + " markdown children but no `_index.md`. " +
			"Hugo auto-generates a flat listing in that case — a wall of links. " +
			"A curated `_index.md` (one paragraph + 3-7 hand-picked docs) is a better " +
			"first impression for newcomers.",
		Fix:  "Add an `_index.md` to the directory with a short intro and the most important docs.",
		Line: 0,
	}
}

// itoa is a tiny int-to-string helper to avoid importing strconv just
// for one call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
