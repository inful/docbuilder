package lint

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"

	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return "", false
	}
	frontmatter := content[4 : endIdx+4]
	for line := range strings.SplitSeq(frontmatter, "\n") {
		trim := strings.TrimSpace(line)
		after, ok := strings.CutPrefix(trim, "uid:")
		if !ok {
			continue
		}
		val := strings.TrimSpace(after)
		if val != "" {
			return val, true
		}
		return "", false
	}
	return "", false
}

func addUIDIfMissingWithValue(content, uid string) (string, bool) {
	if strings.TrimSpace(uid) == "" {
		return content, false
	}

	if !strings.HasPrefix(content, "---\n") {
		fm := "---\nuid: " + uid + "\n---\n\n"
		return fm + content, true
	}

	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return content, false
	}

	frontmatter := content[4 : endIdx+4]
	body := content[endIdx+9:]

	lines := make([]string, 0, 8)
	for line := range strings.SplitSeq(frontmatter, "\n") {
		lines = append(lines, line)
		if _, ok := strings.CutPrefix(strings.TrimSpace(line), "uid:"); ok {
			return content, false
		}
	}

	kept := make([]string, 0, len(lines)+1)
	inserted := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		kept = append(kept, line)
		if !inserted && strings.HasPrefix(trim, "fingerprint:") {
			kept = append(kept, "uid: "+uid)
			inserted = true
		}
	}
	if !inserted {
		out := make([]string, 0, len(kept)+1)
		added := false
		for _, line := range kept {
			trim := strings.TrimSpace(line)
			if !added && trim != "" {
				out = append(out, "uid: "+uid)
				added = true
			}
			out = append(out, line)
		}
		if !added {
			out = append(out, "uid: "+uid)
		}
		kept = out
	}

	newFM := strings.TrimSpace(strings.Join(kept, "\n"))
	if newFM == "" {
		newFM = "uid: " + uid
	}
	return "---\n" + newFM + "\n---\n" + body, true
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
		op.Error = foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem,
			"failed to read file for UID update").
			WithContext("file", filePath).
			Build()
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
		op.Error = foundationerrors.WrapError(statErr, foundationerrors.CategoryFileSystem,
			"failed to stat file for UID update").
			WithContext("file", filePath).
			Build()
		return op
	}

	if writeErr := os.WriteFile(filePath, []byte(updated), info.Mode().Perm()); writeErr != nil {
		op.Success = false
		op.Error = foundationerrors.WrapError(writeErr, foundationerrors.CategoryFileSystem,
			"failed to write file for UID update").
			WithContext("file", filePath).
			Build()
		return op
	}

	return op
}

func addUIDIfMissing(content string) (string, bool) {
	uid := uuid.NewString()

	if !strings.HasPrefix(content, "---\n") {
		fm := "---\nuid: " + uid + "\n---\n\n"
		return fm + content, true
	}

	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		// Malformed frontmatter; don't try to guess.
		return content, false
	}

	frontmatter := content[4 : endIdx+4]
	body := content[endIdx+9:]

	lines := make([]string, 0, 8)
	for line := range strings.SplitSeq(frontmatter, "\n") {
		lines = append(lines, line)
		if _, ok := strings.CutPrefix(strings.TrimSpace(line), "uid:"); ok {
			return content, false
		}
	}

	// Insert uid near the top, after any existing fingerprint line if present,
	// to keep frontmatter stable and readable.
	kept := make([]string, 0, len(lines)+1)
	inserted := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		kept = append(kept, line)
		if !inserted && strings.HasPrefix(trim, "fingerprint:") {
			kept = append(kept, "uid: "+uid)
			inserted = true
		}
	}
	if !inserted {
		// Put it at the top (after any leading empties).
		out := make([]string, 0, len(kept)+1)
		added := false
		for _, line := range kept {
			trim := strings.TrimSpace(line)
			if !added && trim != "" {
				out = append(out, "uid: "+uid)
				added = true
			}
			out = append(out, line)
		}
		if !added {
			out = append(out, "uid: "+uid)
		}
		kept = out
	}

	newFM := strings.TrimSpace(strings.Join(kept, "\n"))
	if newFM == "" {
		newFM = "uid: " + uid
	}
	return "---\n" + newFM + "\n---\n" + body, true
}
