package lint

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
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

	fields, body, had, style, err := frontmatterops.Read([]byte(content))
	if err != nil {
		return content, false
	}
	if style.Newline == "" {
		style.Newline = "\n"
	}
	if fields == nil {
		fields = map[string]any{}
	}

	uidChanged, err := frontmatterops.EnsureUIDValue(fields, uid)
	if err != nil {
		return content, false
	}
	if !uidChanged {
		return content, false
	}

	if !had {
		had = true
		if len(body) > 0 && !bytes.HasPrefix(body, []byte(style.Newline)) {
			body = append([]byte(style.Newline), body...)
		} else if len(body) == 0 {
			body = append([]byte(style.Newline), body...)
		}
	}

	out, err := frontmatterops.Write(fields, body, had, style)
	if err != nil {
		return content, false
	}
	return string(out), true
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
		op.Error = fmt.Errorf("stat file for uid update: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, []byte(updated), info.Mode().Perm()); writeErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("write file for uid update: %w", writeErr)
		return op
	}

	return op
}

func addUIDIfMissing(content string) (string, bool) {
	fields, body, had, style, err := frontmatterops.Read([]byte(content))
	if err != nil {
		// Malformed frontmatter; don't try to guess.
		return content, false
	}
	if style.Newline == "" {
		style.Newline = "\n"
	}
	if fields == nil {
		fields = map[string]any{}
	}

	uid, uidChanged, err := frontmatterops.EnsureUID(fields)
	if err != nil || !uidChanged {
		return content, false
	}

	// Best-effort: if this fails, keep UID but skip alias.
	_, _ = frontmatterops.EnsureUIDAlias(fields, uid)

	if !had {
		had = true
		if len(body) > 0 && !bytes.HasPrefix(body, []byte(style.Newline)) {
			body = append([]byte(style.Newline), body...)
		} else if len(body) == 0 {
			body = append([]byte(style.Newline), body...)
		}
	}

	out, err := frontmatterops.Write(fields, body, had, style)
	if err != nil {
		return content, false
	}
	return string(out), true
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
		op.Error = fmt.Errorf("stat file for uid alias update: %w", statErr)
		return op
	}

	if writeErr := os.WriteFile(filePath, []byte(updated), info.Mode().Perm()); writeErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("write file for uid alias update: %w", writeErr)
		return op
	}

	return op
}

func addUIDAliasIfMissing(content, uid string) (string, bool) {
	fields, body, had, style, err := frontmatterops.Read([]byte(content))
	if err != nil || !had {
		return content, false
	}
	if style.Newline == "" {
		style.Newline = "\n"
	}

	changed, err := frontmatterops.EnsureUIDAlias(fields, uid)
	if err != nil || !changed {
		return content, false
	}

	out, err := frontmatterops.Write(fields, body, had, style)
	if err != nil {
		return content, false
	}
	return string(out), true
}
