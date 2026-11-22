package frontmatterops

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// UIDAliasPath returns the canonical alias path of a uid.
func UIDAliasPath(uid string) string {
	return "/_uid/" + uid + "/"
}

// ExtractUID returns the uid of a markdown document, and whether it
// has one.
func ExtractUID(content []byte) (string, bool) {
	fields, _, had, _, err := Read(content)
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

// EnsureUIDInContent ensures a markdown document has a uid and the
// matching alias. The document is returned unchanged when it already
// has a uid, or when its frontmatter cannot be read.
func EnsureUIDInContent(content []byte) ([]byte, bool) {
	fields, body, had, style, err := Read(content)
	if err != nil {
		// Malformed frontmatter; don't try to guess.
		return content, false
	}
	style = withNewline(style)
	if fields == nil {
		fields = map[string]any{}
	}

	uid, uidChanged, err := EnsureUID(fields)
	if err != nil || !uidChanged {
		return content, false
	}

	// Best-effort: if this fails, keep UID but skip alias.
	_, _ = EnsureUIDAlias(fields, uid)

	body, had = withFrontmatter(body, had, style)
	out, err := Write(fields, body, had, style)
	if err != nil {
		return content, false
	}
	return out, true
}

// EnsureUIDValueInContent ensures a markdown document has the given
// uid. The document is returned unchanged when it already has a uid,
// or when its frontmatter cannot be read.
func EnsureUIDValueInContent(content []byte, uid string) ([]byte, bool) {
	if strings.TrimSpace(uid) == "" {
		return content, false
	}

	fields, body, had, style, err := Read(content)
	if err != nil {
		return content, false
	}
	style = withNewline(style)
	if fields == nil {
		fields = map[string]any{}
	}

	uidChanged, err := EnsureUIDValue(fields, uid)
	if err != nil || !uidChanged {
		return content, false
	}

	body, had = withFrontmatter(body, had, style)
	out, err := Write(fields, body, had, style)
	if err != nil {
		return content, false
	}
	return out, true
}

// EnsureUIDAliasInContent ensures a markdown document has the alias
// of the given uid. The document is returned unchanged when it has no
// frontmatter, already has the alias, or its frontmatter cannot be
// read.
func EnsureUIDAliasInContent(content []byte, uid string) ([]byte, bool) {
	fields, body, had, style, err := Read(content)
	if err != nil || !had {
		return content, false
	}

	changed, err := EnsureUIDAlias(fields, uid)
	if err != nil || !changed {
		return content, false
	}

	out, err := Write(fields, body, had, style)
	if err != nil {
		return content, false
	}
	return out, true
}

// withNewline returns the style with a newline set, for documents
// whose frontmatter is written for the first time.
func withNewline(style frontmatter.Style) frontmatter.Style {
	if style.Newline == "" {
		style.Newline = "\n"
	}
	return style
}

// withFrontmatter prepares the body of a document that gains
// frontmatter: the closing delimiter needs a newline after it.
func withFrontmatter(body []byte, had bool, style frontmatter.Style) ([]byte, bool) {
	if had {
		return body, had
	}
	if len(body) > 0 && !bytes.HasPrefix(body, []byte(style.Newline)) {
		body = append([]byte(style.Newline), body...)
	} else if len(body) == 0 {
		body = append([]byte(style.Newline), body...)
	}
	return body, true
}

// EnsureUID ensures fields contains a uid.
//
// It only generates a new uid when the key is missing.
func EnsureUID(fields map[string]any) (uidStr string, changed bool, err error) {
	if fields == nil {
		return "", false, errors.New("fields map is nil")
	}

	if v, ok := fields["uid"]; ok {
		return strings.TrimSpace(fmt.Sprint(v)), false, nil
	}

	uidStr = uuid.NewString()
	fields["uid"] = uidStr
	return uidStr, true, nil
}

// EnsureUIDValue ensures fields contains a uid with the provided value.
//
// It only sets the uid when the key is missing.
func EnsureUIDValue(fields map[string]any, uidStr string) (changed bool, err error) {
	if fields == nil {
		return false, errors.New("fields map is nil")
	}

	uidStr = strings.TrimSpace(uidStr)
	if uidStr == "" {
		return false, errors.New("uid is empty")
	}

	if _, ok := fields["uid"]; ok {
		return false, nil
	}

	fields["uid"] = uidStr
	return true, nil
}

// EnsureUIDAlias ensures fields.aliases contains "/_uid/<uid>/".
//
// It follows the existing lint/fix semantics closely: if aliases already contains
// the expected alias (even as a single string), it reports changed=false.
func EnsureUIDAlias(fields map[string]any, uid string) (changed bool, err error) {
	if fields == nil {
		return false, errors.New("fields map is nil")
	}

	uid = strings.TrimSpace(uid)
	if uid == "" {
		return false, errors.New("uid is empty")
	}

	expected := UIDAliasPath(uid)

	aliases, ok := fields["aliases"]
	if !ok || aliases == nil {
		fields["aliases"] = []string{expected}
		return true, nil
	}

	appendIfMissing := func(list []string) (bool, []string) {
		if slices.Contains(list, expected) {
			return false, list
		}
		return true, append(list, expected)
	}

	switch v := aliases.(type) {
	case []string:
		aliasesChanged, out := appendIfMissing(v)
		if aliasesChanged {
			fields["aliases"] = out
		}
		return aliasesChanged, nil
	case []any:
		out := make([]string, 0, len(v)+1)
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		aliasesChanged, out := appendIfMissing(out)
		if aliasesChanged {
			fields["aliases"] = out
		}
		return aliasesChanged, nil
	case string:
		if strings.TrimSpace(v) == expected {
			// Preserve existing lint/fix behavior: normalize to list, even if not counted as a change.
			fields["aliases"] = []string{expected}
			return false, nil
		}
		fields["aliases"] = []string{v, expected}
		return true, nil
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" {
			fields["aliases"] = []string{expected}
			return true, nil
		}
		if s == expected {
			fields["aliases"] = []string{expected}
			return false, nil
		}
		fields["aliases"] = []string{s, expected}
		return true, nil
	}
}
