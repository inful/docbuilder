package frontmatterops

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

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

	expected := "/_uid/" + uid + "/"

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
