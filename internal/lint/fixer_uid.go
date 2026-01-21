package lint

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"

	"github.com/google/uuid"
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
	fmRaw, _, had, _, err := frontmatter.Split([]byte(content))
	if err != nil || !had {
		return "", false
	}

	fm, err := frontmatter.ParseYAML(fmRaw)
	if err != nil {
		return "", false
	}

	val, ok := fm["uid"]
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

	fmRaw, body, had, style, err := frontmatter.Split([]byte(content))
	if err != nil {
		return content, false
	}

	fields := map[string]any{}
	if had {
		fields, err = frontmatter.ParseYAML(fmRaw)
		if err != nil {
			return content, false
		}
	}

	if _, ok := fields["uid"]; ok {
		return content, false
	}
	fields["uid"] = uid

	fmYAML, err := frontmatter.SerializeYAML(fields, style)
	if err != nil {
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

	return string(frontmatter.Join(fmYAML, body, had, style)), true
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
	uid := uuid.NewString()
	return addUIDAndAliasIfMissing(content, uid, true)
}

func addUIDAndAliasIfMissing(content, uid string, includeAlias bool) (string, bool) {
	fmRaw, body, had, style, err := frontmatter.Split([]byte(content))
	if err != nil {
		// Malformed frontmatter; don't try to guess.
		return content, false
	}

	fields := map[string]any{}
	if had {
		fields, err = frontmatter.ParseYAML(fmRaw)
		if err != nil {
			// Malformed YAML; don't try to guess.
			return content, false
		}
	}

	if _, ok := fields["uid"]; ok {
		return content, false
	}

	fields["uid"] = uid
	if includeAlias {
		_ = ensureUIDAlias(fields, uid)
	}

	fmYAML, err := frontmatter.SerializeYAML(fields, style)
	if err != nil {
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

	return string(frontmatter.Join(fmYAML, body, had, style)), true
}

func ensureUIDAlias(fields map[string]any, uid string) bool {
	expected := "/_uid/" + uid + "/"

	aliases, ok := fields["aliases"]
	if !ok || aliases == nil {
		fields["aliases"] = []string{expected}
		return true
	}

	set := func(list []string) (bool, []string) {
		if slices.Contains(list, expected) {
			return false, list
		}
		return true, append(list, expected)
	}

	switch v := aliases.(type) {
	case []string:
		changed, out := set(v)
		if changed {
			fields["aliases"] = out
		}
		return changed
	case []any:
		out := make([]string, 0, len(v)+1)
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		changed, out := set(out)
		if changed {
			fields["aliases"] = out
		}
		return changed
	case string:
		if v == expected {
			fields["aliases"] = []string{v}
			return false
		}
		fields["aliases"] = []string{v, expected}
		return true
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" {
			fields["aliases"] = []string{expected}
			return true
		}
		if s == expected {
			fields["aliases"] = []string{s}
			return false
		}
		fields["aliases"] = []string{s, expected}
		return true
	}
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
	fmRaw, body, had, style, err := frontmatter.Split([]byte(content))
	if err != nil || !had {
		return content, false
	}

	fields, err := frontmatter.ParseYAML(fmRaw)
	if err != nil {
		return content, false
	}

	if changed := ensureUIDAlias(fields, uid); !changed {
		return content, false
	}

	fmYAML, err := frontmatter.SerializeYAML(fields, style)
	if err != nil {
		return content, false
	}
	return string(frontmatter.Join(fmYAML, body, had, style)), true
}
