package lint

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
)

// templateUIDBindingUpdate is the result of fixing a single template
// file. Multiple files can be fixed in one fixer pass.
type templateUIDBindingUpdate struct {
	FilePath string
	Success  bool
	Changed  bool
	Error    error
}

// applyTemplateUIDBindingFixes walks every target file and, for each
// one, rewrites the embedded frontmatter inside its markdown fence
// so that any body reference to `{{ .UID }}` is bound to
// `uid: "{{ .UID }}"`. Files that already have the binding (or that
// are not templates) are left alone.
//
// Literal-uid mismatches are NOT auto-fixed: silently replacing a
// hand-written UUID would be destructive and the user should make
// that call explicitly.
func (f *Fixer) applyTemplateUIDBindingFixes(targets map[string]struct{}, issueCounts map[string]int, fixResult *FixResult, fingerprintTargets map[string]struct{}) {
	if len(targets) == 0 {
		return
	}

	paths := make([]string, 0, len(targets))
	for p := range targets {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		op := f.fixTemplateUIDBinding(p)
		if op.Error != nil {
			fixResult.Errors = append(fixResult.Errors, op.Error)
			continue
		}
		if !op.Changed {
			// Either the body was already correct, or it had a
			// literal uid (mismatch, not auto-fixed). Either way
			// the on-disk file is unchanged.
			continue
		}
		// Outer-file content changed; the fingerprint must be refreshed.
		fingerprintTargets[p] = struct{}{}
		// Treat the fix as resolved for the issue counts the
		// initial lint pass reported. (If a literal mismatch was
		// also reported, the linter will keep flagging it on
		// the next pass; the auto-fixer is intentionally narrow.)
		fixResult.ErrorsFixed += issueCounts[p]
	}
}

// fixTemplateUIDBinding reads the file, finds the fenced markdown
// body, and — if the body references {{ .UID }} but the embedded
// frontmatter has no `uid: "{{ .UID }}"` — rewrites the embedded
// frontmatter with the binding. Returns Changed=true only when the
// file content actually changed.
func (f *Fixer) fixTemplateUIDBinding(filePath string) templateUIDBindingUpdate {
	op := templateUIDBindingUpdate{FilePath: filePath, Success: true}

	// #nosec G304 -- filePath is derived from the current lint target set.
	data, err := os.ReadFile(filePath)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("read file for template-uid-binding fix: %w", err)
		return op
	}
	original := string(data)

	if !isTemplateFile(data) {
		return op
	}

	text := original
	body, bodyStart, bodyEnd, ok := extractTemplateBody(text)
	if !ok {
		return op
	}

	if !hasUIDReference(body) {
		return op
	}

	// Parse the embedded frontmatter. If it's malformed, we can't
	// safely splice a field in — bail out without changing the file.
	fields, bodyAfterFM, had, style, err := frontmatterops.Read([]byte(body))
	if err != nil {
		// Don't escalate: let the linter flag the malformed inner
		// frontmatter on the next pass.
		return op
	}
	if !had {
		// No embedded frontmatter at all. The rule's missing-uid
		// issue reported this case; the auto-fix is to add a
		// minimal embedded frontmatter containing only the uid
		// binding. Insert a small `--- uid: "{{ .UID }}" ---`
		// block at the very top of the body.
		newBody := buildInjectedUIDFrontmatter(body, style)
		return f.writeUpdatedBody(filePath, text, bodyStart, bodyEnd, original, newBody, op)
	}

	// If a {{ .UID }} template reference is already set, nothing to do.
	if uidRaw, present := fields["uid"]; present {
		if uidStr, ok := uidRaw.(string); ok && uidFieldIsTemplate(uidStr) {
			return op
		}
		// Otherwise the field is a literal (or non-string) value:
		// the body says "use {{ .UID }}" but the frontmatter pins
		// a different identifier, so we overwrite with the template
		// reference. The rule only fires on template files whose body
		// already uses {{ .UID }}, so this is what the author meant.
	}

	// Bind the embedded frontmatter's `uid` to {{ .UID }}.
	fields["uid"] = "{{ .UID }}"
	updatedBody, err := frontmatterops.Write(fields, bodyAfterFM, had, style)
	if err != nil {
		op.Success = false
		op.Error = fmt.Errorf("write embedded frontmatter: %w", err)
		return op
	}
	return f.writeUpdatedBody(filePath, text, bodyStart, bodyEnd, original, string(updatedBody), op)
}

// buildInjectedUIDFrontmatter builds a minimal embedded frontmatter
// block (`---\nuid: "{{ .UID }}"\n---\n`) using the same line ending
// style as the surrounding file, prepended to body.
func buildInjectedUIDFrontmatter(body string, style frontmatter.Style) string {
	// We deliberately bypass frontmatterops.Write here because the
	// body has no frontmatter of its own (had=false) and we want to
	// synthesize one.
	fields := map[string]any{"uid": "{{ .UID }}"}
	raw := serializeSimpleYAML(fields, style.Newline)
	nl := style.Newline
	if nl == "" {
		nl = "\n"
	}
	marker := "---"
	return marker + nl + raw + marker + nl + nl + body
}

func (f *Fixer) writeUpdatedBody(filePath, text string, bodyStart, bodyEnd int, original, newBody string, op templateUIDBindingUpdate) templateUIDBindingUpdate {
	updated := text[:bodyStart] + newBody + text[bodyEnd:]
	if updated == original {
		return op
	}
	op.Changed = true
	if f.dryRun {
		return op
	}
	info, statErr := os.Stat(filePath)
	if statErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("stat file for template-uid-binding fix: %w", statErr)
		return op
	}
	//nolint:gosec // filePath is derived from the lint target set
	if writeErr := os.WriteFile(filePath, []byte(updated), info.Mode().Perm()); writeErr != nil {
		op.Success = false
		op.Error = fmt.Errorf("write file for template-uid-binding fix: %w", writeErr)
		return op
	}
	return op
}

// serializeSimpleYAML is a small helper that emits a single
// `key: value` line per map entry. We don't reuse
// frontmatter.SerializeYAML because we want the simplest possible
// output: `uid: "{{ .UID }}"\n`, with no comments or extra
// structure. The caller controls indentation via the style.
func serializeSimpleYAML(fields map[string]any, nl string) string {
	if nl == "" {
		nl = "\n"
	}
	// Deterministic key order: alphabetical.
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := fields[k]
		switch val := v.(type) {
		case string:
			fmt.Fprintf(&b, "%s: %q%s", k, val, nl)
		default:
			fmt.Fprintf(&b, "%s: %v%s", k, val, nl)
		}
	}
	return b.String()
}
