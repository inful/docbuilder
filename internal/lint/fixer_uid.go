package lint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	if !strings.HasPrefix(content, "---\n") {
		lines := []string{"uid: " + uid}
		if includeAlias {
			lines = append(lines, "aliases:", "  - /_uid/"+uid+"/")
		}
		fm := "---\n" + strings.Join(lines, "\n") + "\n---\n\n"
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
	kept, _ := insertUIDInFrontmatter(lines, uid, includeAlias)

	newFM := strings.TrimSpace(strings.Join(kept, "\n"))
	if newFM == "" {
		if includeAlias {
			newFM = "uid: " + uid + "\naliases:\n  - /_uid/" + uid + "/"
		} else {
			newFM = "uid: " + uid
		}
	}
	return "---\n" + newFM + "\n---\n" + body, true
}

// insertUIDInFrontmatter inserts uid (and optionally aliases) into frontmatter lines.
func insertUIDInFrontmatter(lines []string, uid string, includeAlias bool) ([]string, bool) {
	kept := make([]string, 0, len(lines)+2)
	inserted := false

	// Try to insert after fingerprint line
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		kept = append(kept, line)
		if !inserted && strings.HasPrefix(trim, "fingerprint:") {
			kept = append(kept, "uid: "+uid)
			if includeAlias {
				kept = append(kept, "aliases:", "  - /_uid/"+uid+"/")
			}
			inserted = true
		}
	}

	if inserted {
		return kept, true
	}

	// No fingerprint line found, insert at top after any leading empties
	return insertUIDAtTop(kept, uid, includeAlias)
}

// insertUIDAtTop inserts uid at the top of frontmatter, after any leading empty lines.
func insertUIDAtTop(lines []string, uid string, includeAlias bool) ([]string, bool) {
	out := make([]string, 0, len(lines)+2)
	added := false

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if !added && trim != "" {
			out = append(out, "uid: "+uid)
			if includeAlias {
				out = append(out, "aliases:", "  - /_uid/"+uid+"/")
			}
			added = true
		}
		out = append(out, line)
	}

	if !added {
		out = append(out, "uid: "+uid)
		if includeAlias {
			out = append(out, "aliases:", "  - /_uid/"+uid+"/")
		}
		added = true
	}

	return out, added
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
	if !strings.HasPrefix(content, "---\n") {
		return content, false
	}

	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return content, false
	}

	frontmatter := content[4 : endIdx+4]
	body := content[endIdx+9:]

	expectedAlias := "/_uid/" + uid + "/"
	lines := make([]string, 0, 16)
	hasAliases := false
	aliasesIndent := "  "

	for line := range strings.SplitSeq(frontmatter, "\n") {
		lines = append(lines, line)
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "aliases:") {
			hasAliases = true
		}
	}

	// Check if alias already exists
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trim, "- "); ok {
			alias := strings.TrimSpace(after)
			if alias == expectedAlias {
				return content, false // Already has the alias
			}
		}
	}

	// Add the alias
	var kept []string
	var inserted bool

	if !hasAliases {
		kept, inserted = insertAliasesField(lines, expectedAlias, aliasesIndent)
	} else {
		kept, inserted = appendToExistingAliases(lines, expectedAlias, aliasesIndent)
	}

	if !inserted {
		return content, false
	}

	newFM := strings.TrimSpace(strings.Join(kept, "\n"))
	return "---\n" + newFM + "\n---\n" + body, true
}

// insertAliasesField creates a new aliases field after the uid line.
func insertAliasesField(lines []string, expectedAlias, aliasesIndent string) ([]string, bool) {
	kept := make([]string, 0, len(lines)+3)
	inserted := false

	for _, line := range lines {
		kept = append(kept, line)
		trim := strings.TrimSpace(line)
		if !inserted && strings.HasPrefix(trim, "uid:") {
			kept = append(kept, "aliases:", aliasesIndent+"- "+expectedAlias)
			inserted = true
		}
	}

	if !inserted {
		// Add at end of frontmatter
		kept = append(kept, "aliases:", aliasesIndent+"- "+expectedAlias)
		inserted = true
	}

	return kept, inserted
}

// isAliasItem returns true if the trimmed line is an alias list item.
func isAliasItem(trimmedLine string) bool {
	return strings.HasPrefix(trimmedLine, "- ")
}

// appendToExistingAliases adds an alias to an existing aliases field.
func appendToExistingAliases(lines []string, expectedAlias, aliasesIndent string) ([]string, bool) {
	aliasesLineIdx := -1
	lastAliasLineIdx := -1

	// Find aliases section and last alias in it
AliasLoop:
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trim, "aliases:"):
			aliasesLineIdx = i
		case aliasesLineIdx >= 0 && isAliasItem(trim):
			lastAliasLineIdx = i
		case aliasesLineIdx >= 0 && lastAliasLineIdx >= 0 && trim != "" && !strings.HasPrefix(trim, "#") && !isAliasItem(trim):
			// Hit a non-alias field after we found aliases - stop scanning
			break AliasLoop
		}
	}

	if aliasesLineIdx < 0 {
		return nil, false
	}

	// Insert after last alias (or after "aliases:" if no aliases yet)
	insertIdx := lastAliasLineIdx
	if insertIdx < 0 {
		insertIdx = aliasesLineIdx
	}

	// Build result with alias inserted
	kept := make([]string, 0, len(lines)+1)
	kept = append(kept, lines[:insertIdx+1]...)
	kept = append(kept, aliasesIndent+"- "+expectedAlias)
	kept = append(kept, lines[insertIdx+1:]...)

	return kept, true
}
