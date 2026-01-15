package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// applyTaxonomyFixes normalizes tags and categories in frontmatter.
func (f *Fixer) applyTaxonomyFixes(targets map[string]struct{}, taxonomyIssueCounts map[string]int, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
	if len(targets) == 0 {
		return
	}

	paths := make([]string, 0, len(targets))
	for p := range targets {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		ext := strings.ToLower(filepath.Ext(p))
		if ext != docExtensionMarkdown && ext != docExtensionMarkdownLong {
			continue
		}

		op := f.normalizeTaxonomy(p)
		if op.Success {
			fixResult.ErrorsFixed += taxonomyIssueCounts[p]
			// Taxonomy changes modify content, so fingerprints must be refreshed.
			fingerprintTargets[p] = struct{}{}
			continue
		}
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
		}
	}
}

// TaxonomyUpdate represents a taxonomy normalization operation.
type TaxonomyUpdate struct {
	FilePath string
	Success  bool
	Error    error
}

// normalizeTaxonomy normalizes tags and categories in a file's frontmatter.
func (f *Fixer) normalizeTaxonomy(filePath string) TaxonomyUpdate {
	op := TaxonomyUpdate{FilePath: filePath, Success: true}

	// #nosec G304 -- filePath is derived from the current lint/fix target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for taxonomy update: %w", err)
		return op
	}

	updated, changed := normalizeTaxonomyInContent(string(data))
	if !changed {
		return op
	}

	if f.dryRun {
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for taxonomy update: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, []byte(updated), info.Mode().Perm()); writeErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("write file for taxonomy update: %w", writeErr)
		return op
	}

	return op
}

// normalizeTaxonomyInContent normalizes tags and categories in the content.
func normalizeTaxonomyInContent(content string) (string, bool) {
	fm, ok := extractFrontmatter(content)
	if !ok {
		// No frontmatter - nothing to normalize
		return content, false
	}

	// Parse frontmatter as YAML
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(fm), &obj); err != nil {
		// Invalid YAML - can't normalize
		return content, false
	}

	changed := false

	// Normalize tags
	if tagsAny, hasTags := obj["tags"]; hasTags {
		tags := extractStringArray(tagsAny)
		normalizedTags, tagsChanged := normalizeTags(tags)
		if tagsChanged {
			obj["tags"] = normalizedTags
			changed = true
		}
	}

	// Normalize categories
	if categoriesAny, hasCategories := obj["categories"]; hasCategories {
		categories := extractStringArray(categoriesAny)
		normalizedCategories, categoriesChanged := normalizeCategories(categories)
		if categoriesChanged {
			obj["categories"] = normalizedCategories
			changed = true
		}
	}

	if !changed {
		return content, false
	}

	// Reconstruct the content with normalized frontmatter
	updatedFM, err := yaml.Marshal(obj)
	if err != nil {
		// Failed to marshal - return unchanged
		return content, false
	}

	// Extract the body (everything after frontmatter)
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return content, false
	}
	body := content[endIdx+9:]

	// Reconstruct with normalized frontmatter
	return "---\n" + string(updatedFM) + "---\n" + body, true
}
