package suggest

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Index stores deduplicated filesystem-derived suggestion candidates.
type Index struct {
	candidates []string
	docsDir    string
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

	return Index{candidates: candidates, docsDir: docsDir}, nil
}

func addCandidate(entries map[string]struct{}, value string) {
	clean := strings.TrimSpace(filepath.ToSlash(value))
	if clean == "" {
		return
	}
	entries[strings.ToLower(clean)] = struct{}{}
}

// SuggestFromGlob returns ranked suggestions derived from a docs-relative glob pattern.
// Supported pattern semantics:
//   - "dir/*.md" -> stem of matches under dir (e.g., "something" from dir/something.md)
//   - "dir/*/"   -> direct child directory names under dir
func (i Index) SuggestFromGlob(globPattern, input string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	if strings.TrimSpace(i.docsDir) == "" || strings.TrimSpace(globPattern) == "" {
		return nil
	}

	pattern := filepath.ToSlash(strings.TrimSpace(globPattern))
	absPattern := filepath.Join(i.docsDir, filepath.FromSlash(pattern))
	matches, err := filepath.Glob(absPattern)
	if err != nil || len(matches) == 0 {
		return nil
	}

	dirMode := strings.HasSuffix(pattern, "*/")
	values := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		rel, relErr := filepath.Rel(i.docsDir, match)
		if relErr != nil {
			continue
		}
		rel = filepath.ToSlash(rel)

		if dirMode {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			name := strings.ToLower(path.Base(strings.TrimSuffix(rel, "/")))
			if name != "" && name != "." {
				values[name] = struct{}{}
			}
			continue
		}

		stem := extractGlobValue(pattern, rel)
		if stem != "" {
			values[strings.ToLower(stem)] = struct{}{}
		}
	}

	if len(values) == 0 {
		return nil
	}

	candidates := make([]string, 0, len(values))
	for value := range values {
		candidates = append(candidates, value)
	}
	sort.Strings(candidates)
	return rankCandidates(candidates, input, limit)
}

func extractGlobValue(pattern, relMatch string) string {
	patternBase := path.Base(pattern)
	matchBase := path.Base(relMatch)

	if strings.Contains(patternBase, "*") {
		parts := strings.SplitN(patternBase, "*", 2)
		prefix, suffix := parts[0], parts[1]
		if strings.HasPrefix(matchBase, prefix) && strings.HasSuffix(matchBase, suffix) && len(matchBase) >= len(prefix)+len(suffix) {
			return matchBase[len(prefix) : len(matchBase)-len(suffix)]
		}
	}

	ext := path.Ext(matchBase)
	if ext != "" {
		return strings.TrimSuffix(matchBase, ext)
	}
	return matchBase
}

// Suggest returns ranked candidates for input, prefix first then substring matches.
func (i Index) Suggest(input string, limit int) []string {
	if limit <= 0 {
		return nil
	}

	if len(i.candidates) == 0 {
		return nil
	}

	return rankCandidates(i.candidates, input, limit)
}

func rankCandidates(candidates []string, input string, limit int) []string {
	needle := strings.ToLower(strings.TrimSpace(input))

	prefix := make([]string, 0, limit)
	contains := make([]string, 0, limit)
	for _, candidate := range candidates {
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
