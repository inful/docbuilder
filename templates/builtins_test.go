package templates

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// uuidV4Pattern matches the canonical 8-4-4-4-12 hex layout of a UUID.
var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestWithBuiltinTemplateData_AddsDate(t *testing.T) {
	out := WithBuiltinTemplateData(map[string]any{})
	require.Regexp(t, regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`), out["Date"])
}

func TestWithBuiltinTemplateData_AddsDateTime(t *testing.T) {
	out := WithBuiltinTemplateData(map[string]any{})
	require.Regexp(t, regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`), out["DateTime"])
}

func TestWithBuiltinTemplateData_AddsUID(t *testing.T) {
	out := WithBuiltinTemplateData(map[string]any{})
	uid, ok := out["UID"].(string)
	require.True(t, ok, "UID should be a string")
	require.Regexp(t, uuidV4Pattern, uid)
}

func TestWithBuiltinTemplateData_PreservesExistingUID(t *testing.T) {
	in := map[string]any{"UID": "my-custom-id"}
	out := WithBuiltinTemplateData(in)
	require.Equal(t, "my-custom-id", out["UID"])
}

func TestWithBuiltinTemplateData_GeneratesUniqueUIDs(t *testing.T) {
	// Two separate calls should produce different UIDs.
	// (Within one call the same UID is shared across Date, DateTime, UID keys
	// when callers reuse the returned map.)
	out1 := WithBuiltinTemplateData(map[string]any{})
	out2 := WithBuiltinTemplateData(map[string]any{})
	require.NotEqual(t, out1["UID"], out2["UID"])
}

func TestWithBuiltinTemplateData_PreservesAllExisting(t *testing.T) {
	in := map[string]any{
		"Date":     "2020-01-01",
		"DateTime": "2020-01-01T00:00:00Z",
		"UID":      "preserved",
		"Title":    "user value",
	}
	out := WithBuiltinTemplateData(in)
	require.Equal(t, "2020-01-01", out["Date"])
	require.Equal(t, "2020-01-01T00:00:00Z", out["DateTime"])
	require.Equal(t, "preserved", out["UID"])
	require.Equal(t, "user value", out["Title"])
}

func TestWithBuiltinTemplateData_HandlesNil(t *testing.T) {
	out := WithBuiltinTemplateData(nil)
	require.NotNil(t, out)
	require.Regexp(t, regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`), out["Date"])
	require.Regexp(t, regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`), out["DateTime"])
	require.Regexp(t, uuidV4Pattern, out["UID"])
}
