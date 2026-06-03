package config

import (
	"reflect"
	"testing"
)

// TestSidebarConfigNormalize_GroupByLowercasesKeysAndFields pins the
// normalisation: map keys (canonical category names) and field names
// are lowercased in place so users can write them in any case.
// Without this, "Minutes" or "Project" would silently fail to match
// the canonical lowercase keys / front-matter field names and the
// categories menu would fall back to Repository-based grouping.
func TestSidebarConfigNormalize_GroupByLowercasesKeysAndFields(t *testing.T) {
	s := &SidebarConfig{
		GroupBy: map[string][]string{
			"Minutes":    {"Project"},
			"OPERATIONS": {"TEAM", "Sprint"},
		},
	}
	res := &NormalizationResult{}
	s.Normalize(res)

	want := map[string][]string{
		"minutes":    {"project"},
		"operations": {"team", "sprint"},
	}
	if !reflect.DeepEqual(s.GroupBy, want) {
		t.Errorf("Normalize did not lowercase correctly:\n got: %v\nwant: %v", s.GroupBy, want)
	}
}

// TestSidebarConfigNormalize_GroupByDropsEmptyFields asserts that an
// empty string in the field list is dropped (it would otherwise
// cause an unreadable entry on every doc's GroupFields).
func TestSidebarConfigNormalize_GroupByDropsEmptyFields(t *testing.T) {
	s := &SidebarConfig{
		GroupBy: map[string][]string{
			"minutes": {"project", "", "team"},
		},
	}
	res := &NormalizationResult{}
	s.Normalize(res)

	want := []string{"project", "team"}
	if !reflect.DeepEqual(s.GroupBy["minutes"], want) {
		t.Errorf("Normalize did not drop empty fields:\n got: %v\nwant: %v", s.GroupBy["minutes"], want)
	}
}

// TestSidebarConfigNormalize_GroupByNilIsNoop ensures Normalize
// doesn't panic on a nil GroupBy and leaves the field nil.
func TestSidebarConfigNormalize_GroupByNilIsNoop(t *testing.T) {
	s := &SidebarConfig{Mode: SidebarModeCategories}
	res := &NormalizationResult{}
	s.Normalize(res)
	if s.GroupBy != nil {
		t.Errorf("expected GroupBy to remain nil; got %v", s.GroupBy)
	}
}
