package lint

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// RenameSource records where a rename mapping came from.
// This is used to distinguish fixer-driven renames from Git-detected renames.
//
// Note: Today only RenameSourceFixer is produced; other values will be used by ADR-012.
type RenameSource string

const (
	RenameSourceFixer          RenameSource = "fixer"
	RenameSourceGitUncommitted RenameSource = "git-uncommitted"
	RenameSourceGitHistory     RenameSource = "git-history"
)

// RenameMapping represents a single old->new mapping.
// Paths must be absolute on disk.
type RenameMapping struct {
	OldAbs string
	NewAbs string
	Source RenameSource
}

// NormalizeRenameMappings validates, filters, de-duplicates, and sorts rename mappings.
//
// - Requires OldAbs/NewAbs to be absolute paths.
// - If docsRoots is non-empty, keeps only mappings where both paths are within any docs root.
// - Removes exact duplicates.
// - Sorts deterministically by OldAbs, then NewAbs, then Source.
func NormalizeRenameMappings(mappings []RenameMapping, docsRoots []string) ([]RenameMapping, error) {
	if len(mappings) == 0 {
		return nil, nil
	}

	absDocsRoots := make([]string, 0, len(docsRoots))
	for _, root := range docsRoots {
		if root == "" {
			continue
		}
		if !filepath.IsAbs(root) {
			return nil, fmt.Errorf("docs root must be an absolute path: %q", root)
		}
		absDocsRoots = append(absDocsRoots, filepath.Clean(root))
	}

	filtered := make([]RenameMapping, 0, len(mappings))
	for _, m := range mappings {
		if !filepath.IsAbs(m.OldAbs) || !filepath.IsAbs(m.NewAbs) {
			return nil, fmt.Errorf("rename mapping paths must be absolute: old=%q new=%q", m.OldAbs, m.NewAbs)
		}
		m.OldAbs = filepath.Clean(m.OldAbs)
		m.NewAbs = filepath.Clean(m.NewAbs)

		if len(absDocsRoots) > 0 {
			inScope := false
			for _, root := range absDocsRoots {
				if isWithinDir(m.OldAbs, root) && isWithinDir(m.NewAbs, root) {
					inScope = true
					break
				}
			}
			if !inScope {
				continue
			}
		}

		filtered = append(filtered, m)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].OldAbs != filtered[j].OldAbs {
			return filtered[i].OldAbs < filtered[j].OldAbs
		}
		if filtered[i].NewAbs != filtered[j].NewAbs {
			return filtered[i].NewAbs < filtered[j].NewAbs
		}
		return string(filtered[i].Source) < string(filtered[j].Source)
	})

	deduped := make([]RenameMapping, 0, len(filtered))
	seen := make(map[string]struct{}, len(filtered))
	for _, m := range filtered {
		key := strings.Join([]string{m.OldAbs, "\x00", m.NewAbs, "\x00", string(m.Source)}, "")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, m)
	}

	return deduped, nil
}

func isWithinDir(absPath, absDir string) bool {
	absPath = filepath.Clean(absPath)
	absDir = filepath.Clean(absDir)

	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	// If rel starts with "..", absPath is outside absDir.
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
