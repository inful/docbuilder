package suggest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Index stores deduplicated filesystem-derived suggestion candidates.
type Index struct {
	candidates []string
}

// BuildFromDocs builds a suggestion index by scanning a docs directory.
func BuildFromDocs(docsDir string) (Index, error) {
	if strings.TrimSpace(docsDir) == "" {
		return Index{}, nil
	}

	entries := make(map[string]struct{})
	err := filepath.WalkDir(docsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(docsDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "" {
			return nil
		}

		base := d.Name()
		if strings.HasPrefix(base, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			addCandidate(entries, base)
			addCandidate(entries, rel)
			return nil
		}

		ext := strings.ToLower(filepath.Ext(base))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		stem := strings.TrimSuffix(base, filepath.Ext(base))
		stem = strings.TrimSuffix(stem, ".template")
		if stem != "_index" && stem != "index" {
			addCandidate(entries, stem)
		}
		relNoExt := strings.TrimSuffix(rel, filepath.Ext(rel))
		relNoExt = strings.TrimSuffix(relNoExt, ".template")
		if relNoExt != "" {
			addCandidate(entries, relNoExt)
		}
		return nil
	})
	if err != nil {
		return Index{}, err
	}

	candidates := make([]string, 0, len(entries))
	for value := range entries {
		candidates = append(candidates, value)
	}
	sort.Strings(candidates)

	return Index{candidates: candidates}, nil
}

func addCandidate(entries map[string]struct{}, value string) {
	clean := strings.TrimSpace(filepath.ToSlash(value))
	if clean == "" {
		return
	}
	entries[strings.ToLower(clean)] = struct{}{}
}

// Suggest returns ranked candidates for input, prefix first then substring matches.
func (i Index) Suggest(input string, limit int) []string {
	if limit <= 0 {
		return nil
	}

	needle := strings.ToLower(strings.TrimSpace(input))
	if len(i.candidates) == 0 {
		return nil
	}

	prefix := make([]string, 0, limit)
	contains := make([]string, 0, limit)
	for _, candidate := range i.candidates {
		if needle == "" || strings.HasPrefix(candidate, needle) {
			prefix = append(prefix, candidate)
			continue
		}
		if strings.Contains(candidate, needle) {
			contains = append(contains, candidate)
		}
	}

	result := make([]string, 0, limit)
	for _, value := range prefix {
		result = append(result, value)
		if len(result) == limit {
			return result
		}
	}
	for _, value := range contains {
		result = append(result, value)
		if len(result) == limit {
			return result
		}
	}
	return result
}
