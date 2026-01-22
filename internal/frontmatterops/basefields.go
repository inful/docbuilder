package frontmatterops

import (
	"strings"
	"time"
)

// EnsureTypeDocs sets type: docs when missing (or nil).
func EnsureTypeDocs(fields map[string]any) (changed bool) {
	if fields == nil {
		return false
	}

	if v, ok := fields["type"]; ok && v != nil {
		return false
	}

	fields["type"] = "docs"
	return true
}

// EnsureTitle sets title to fallback when missing or empty/whitespace.
func EnsureTitle(fields map[string]any, fallback string) (changed bool) {
	if fields == nil {
		return false
	}

	v, ok := fields["title"]
	if !ok || v == nil {
		fields["title"] = fallback
		return true
	}

	s, ok := v.(string)
	if !ok {
		return false
	}

	if strings.TrimSpace(s) == "" {
		fields["title"] = fallback
		return true
	}

	return false
}

// EnsureDate sets date when missing (or nil).
//
// If commitDate is non-zero, it is used; otherwise now is used.
//
// Format matches the existing Hugo pipeline behavior: "2006-01-02T15:04:05-07:00".
func EnsureDate(fields map[string]any, commitDate time.Time, now time.Time) (changed bool) {
	if fields == nil {
		return false
	}

	if v, ok := fields["date"]; ok && v != nil {
		return false
	}

	t := commitDate
	if t.IsZero() {
		t = now
	}

	fields["date"] = t.Format("2006-01-02T15:04:05-07:00")
	return true
}
