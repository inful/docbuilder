package lint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"

	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

func preserveUIDAcrossContentRewrite(original, updated string) string {
	uid, hasUID := extractUIDFromFrontmatter(original)
	if !hasUID {
		return updated
	}

	withUID, changed := addUIDIfMissingWithValue(updated, uid)
	if !changed {
		return updated
	}
	return withUID
}

func extractUIDFromFrontmatter(content string) (string, bool) {
	fields, _, had, _, err := frontmatterops.Read([]byte(content))
	if err != nil || !had {
		return "", false
	}

	val, ok := fields["uid"]
	if !ok {
		return "", false
	}

	s := strings.TrimSpace(fmt.Sprint(val))
	if s == "" {
		return "", false
	}
	return s, true
}

func addUIDIfMissingWithValue(content, uid string) (string, bool) {
	if strings.TrimSpace(uid) == "" {
		return content, false
	}
	out, changed, err := frontmatterops.UpsertFields(content, true, func(fields map[string]any) (bool, error) {
		return frontmatterops.EnsureUIDValue(fields, uid)
	})
	if err != nil {
		return content, false
	}
	return out, changed
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
		op.Error = derrors.WrapError(err, derrors.CategoryFileSystem, "read file for uid update").Build()
		return op
	}

	updated, changed := addUIDIfMissing(string(data))
	if !changed {
		return op
	}

	if f.dryRun {
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = derrors.WrapError(statErr, derrors.CategoryFileSystem, "stat file for uid update").Build()
		return op
	}

	if writeErr := os.WriteFile(filePath, []byte(updated), info.Mode().Perm()); writeErr != nil { //nolint:gosec // filePath is derived from the lint target set
		op.Success = false
		op.Error = derrors.WrapError(writeErr, derrors.CategoryFileSystem, "write file for uid update").Build()
		return op
	}

	return op
}

func addUIDIfMissing(content string) (string, bool) {
	out, changed, err := frontmatterops.UpsertFields(content, true, func(fields map[string]any) (bool, error) {
		uid, changed, err := frontmatterops.EnsureUID(fields)
		if err != nil || !changed {
			return false, err
		}
		// Best-effort: if alias insertion fails, keep the UID we just
		// wrote and skip the alias. The fix run will surface that as a
		// separate issue on the next pass.
		_, _ = frontmatterops.EnsureUIDAlias(fields, uid)
		return true, nil
	})
	if err != nil {
		return content, false
	}
	return out, changed
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
		op.Error = derrors.WrapError(err, derrors.CategoryFileSystem, "read file for uid alias update").Build()
		return op
	}

	uid, hasUID := extractUIDFromFrontmatter(string(data))
	if !hasUID || uid == "" {
		// Should not happen if linter ran correctly, but be defensive
		op.Success = false
		op.Error = errors.New("cannot add alias: uid not found in frontmatter")
		return op
	}

	updated, changed := addUIDAliasIfMissing(string(data), uid)
	if !changed {
		return op
	}

	if f.dryRun {
		return op
	}

	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = derrors.WrapError(statErr, derrors.CategoryFileSystem, "stat file for uid alias update").Build()
		return op
	}

	if writeErr := os.WriteFile(filePath, []byte(updated), info.Mode().Perm()); writeErr != nil { //nolint:gosec // filePath is derived from the lint target set
		op.Success = false
		op.Error = derrors.WrapError(writeErr, derrors.CategoryFileSystem, "write file for uid alias update").Build()
		return op
	}

	return op
}

func addUIDAliasIfMissing(content, uid string) (string, bool) {
	// Aliases can only be inserted when the doc already has a
	// frontmatter block (the alias points at the existing UID; without
	// a UID there's nothing to alias). Pass createIfMissing=false so
	// UpsertFields returns (content, false, nil) on docs that have no
	// frontmatter -- exactly the prior behavior.
	out, changed, err := frontmatterops.UpsertFields(content, false, func(fields map[string]any) (bool, error) {
		return frontmatterops.EnsureUIDAlias(fields, uid)
	})
	if err != nil {
		return content, false
	}
	return out, changed
}
