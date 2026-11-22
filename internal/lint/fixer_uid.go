package lint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
)

func preserveUIDAcrossContentRewrite(original, updated string) string {
	uid, hasUID := frontmatterops.ExtractUID([]byte(original))
	if !hasUID {
		return updated
	}

	withUID, changed := frontmatterops.EnsureUIDValueInContent([]byte(updated), uid)
	if !changed {
		return updated
	}
	return string(withUID)
}

func (f *Fixer) applyUIDFixes(targets map[string]struct{}, uidIssueCounts map[string]int, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
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

		op := f.ensureFrontmatterUID(p)
		if op.Success {
			fixResult.ErrorsFixed += uidIssueCounts[p]
			// UID insertion changes content, so fingerprints must be refreshed.
			fingerprintTargets[p] = struct{}{}
			continue
		}
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
		}
	}
}

type UIDUpdate struct {
	FilePath string
	Success  bool
	Error    error
}

func (f *Fixer) ensureFrontmatterUID(filePath string) UIDUpdate {
	op := UIDUpdate{FilePath: filePath, Success: true}

	// #nosec G304 -- filePath is derived from the current lint/fix target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for uid update: %w", err)
		return op
	}

	updated, changed := frontmatterops.EnsureUIDInContent(data)
	if !changed {
		return op
	}

	if f.dryRun {
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for uid update: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, updated, info.Mode().Perm()); writeErr != nil { //nolint:gosec // filePath is derived from the lint target set
		op.Success = false
		op.Error = fmt.Errorf("write file for uid update: %w", writeErr)
		return op
	}

	return op
}

func (f *Fixer) applyUIDAliasesFixes(targets map[string]struct{}, uidAliasIssueCounts map[string]int, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
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

		op := f.ensureFrontmatterUIDAlias(p)
		if op.Success {
			fixResult.ErrorsFixed += uidAliasIssueCounts[p]
			// Alias insertion changes content, so fingerprints must be refreshed.
			fingerprintTargets[p] = struct{}{}
			continue
		}
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
		}
	}
}

func (f *Fixer) ensureFrontmatterUIDAlias(filePath string) UIDUpdate {
	op := UIDUpdate{FilePath: filePath, Success: true}

	// #nosec G304 -- filePath is derived from the current lint/fix target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for uid alias update: %w", err)
		return op
	}

	uid, hasUID := frontmatterops.ExtractUID(data)
	if !hasUID || uid == "" {
		// Should not happen if linter ran correctly, but be defensive
		op.Success = false
		op.Error = errors.New("cannot add alias: uid not found in frontmatter")
		return op
	}

	updated, changed := frontmatterops.EnsureUIDAliasInContent(data, uid)
	if !changed {
		return op
	}

	if f.dryRun {
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for uid alias update: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, updated, info.Mode().Perm()); writeErr != nil { //nolint:gosec // filePath is derived from the lint target set
		op.Success = false
		op.Error = fmt.Errorf("write file for uid alias update: %w", writeErr)
		return op
	}

	return op
}
