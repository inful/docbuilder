package frontmatterops

import (
	"bytes"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PatchFields writes the given fields into the raw frontmatter bytes.
//
// The frontmatter is not parsed and written again: the comments of the
// file and the order of its keys survive the fix, and the diff of the
// fix is the field that was added or changed (ADR-014).
//
// A field that is already there is replaced where it is; a field that
// is not there is added at the end of the frontmatter.
func PatchFields(fm []byte, fields map[string]any, newline string) []byte {
	if newline == "" {
		newline = "\n"
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		fm = patchField(fm, key, fields[key], newline)
	}
	return fm
}

// patchField writes one field into the raw frontmatter bytes.
func patchField(fm []byte, key string, value any, newline string) []byte {
	rendered, err := renderField(key, value)
	if err != nil {
		return fm
	}
	// The YAML is written with "\n"; the file has its own line ending.
	rendered = strings.ReplaceAll(rendered, "\n", newline)
	rendered = strings.TrimRight(rendered, "\n")

	lines := strings.Split(string(fm), "\n")
	for i, line := range lines {
		name, ok := topLevelKey(line)
		if !ok || name != key {
			continue
		}
		// The block of the key ends at the next line that is not
		// indented: the items of a list, the lines of a nested map.
		end := i + 1
		for end < len(lines) {
			next := lines[end]
			if strings.TrimSpace(next) == "" || !isIndented(next) {
				break
			}
			end++
		}

		out := make([]string, 0, len(lines)+2)
		out = append(out, lines[:i]...)
		out = append(out, rendered)
		out = append(out, lines[end:]...)
		return []byte(strings.Join(out, "\n"))
	}

	// The key is not there: the field is added at the end.
	text := string(fm)
	if text != "" && !strings.HasSuffix(text, newline) {
		text += newline
	}
	return []byte(text + rendered + newline)
}

// topLevelKey returns the name of the key of a line of the frontmatter.
// A line that is indented belongs to the key above it.
func topLevelKey(line string) (string, bool) {
	if line == "" || isIndented(line) || strings.HasPrefix(strings.TrimSpace(line), "#") {
		return "", false
	}
	name, _, ok := strings.Cut(line, ":")
	if !ok {
		return "", false
	}
	return strings.TrimSpace(name), true
}

// isIndented reports whether the line belongs to the key above it.
func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

// renderField renders a field as YAML.
func renderField(key string, value any) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(map[string]any{key: value}); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
