package templates

import (
	"maps"
	"time"

	"github.com/google/uuid"
)

// Builtin template variables that the docbuilder engine always makes
// available in the data map handed to text/template, regardless of what
// the user supplied in their template schema or overrides.
const (
	// BuiltinKeyDate is the YYYY-MM-DD (UTC) string injected when the
	// template data does not already define a "Date" field.
	BuiltinKeyDate = "Date"
	// BuiltinKeyDateTime is the RFC 3339 (UTC) string injected when the
	// template data does not already define a "DateTime" field.
	BuiltinKeyDateTime = "DateTime"
	// BuiltinKeyUID is the UUID v4 string injected when the template data
	// does not already define a "UID" field. This is the new document's
	// uid, which templates can reference to embed stable identifiers
	// (for example a copyable `/_uid/{{ .UID }}/` link in the body).
	//
	// For the value to match the uid that the lint fixer writes into the
	// document's frontmatter, the template body MUST include
	// `uid: "{{ .UID }}"` in its frontmatter. Otherwise the fixer will
	// generate its own uid and the body's references will drift.
	BuiltinKeyUID = "UID"
)

// WithBuiltinTemplateData returns data augmented with the docbuilder
// builtin template variables: Date (YYYY-MM-DD, UTC), DateTime
// (RFC 3339, UTC), and UID (UUID v4).
//
// Existing keys in data are preserved — builtins are only injected when
// the corresponding key is absent. Callers that need the same UID to
// appear in BOTH the rendered output path AND the rendered body (the
// common case for "create from template") should call this function
// once and pass the returned map to every renderer, instead of letting
// each renderer generate its own UID.
//
// A nil input is treated as an empty map.
func WithBuiltinTemplateData(data map[string]any) map[string]any {
	needDate := data == nil
	needDateTime := data == nil
	needUID := data == nil
	if data != nil {
		if _, ok := data[BuiltinKeyDate]; !ok {
			needDate = true
		}
		if _, ok := data[BuiltinKeyDateTime]; !ok {
			needDateTime = true
		}
		if _, ok := data[BuiltinKeyUID]; !ok {
			needUID = true
		}
	}

	if !needDate && !needDateTime && !needUID {
		return data
	}

	out := make(map[string]any, len(data)+3)
	maps.Copy(out, data)

	now := time.Now().UTC()
	if needDate {
		out[BuiltinKeyDate] = now.Format("2006-01-02")
	}
	if needDateTime {
		out[BuiltinKeyDateTime] = now.Format(time.RFC3339)
	}
	if needUID {
		out[BuiltinKeyUID] = uuid.NewString()
	}

	return out
}
