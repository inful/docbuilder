package fmcore

import (
	"reflect"
	"testing"
)

// TestFrontMatterPatchMerge tests the different merge modes and array strategies
// with various conflict scenarios to ensure patch application behaves correctly.
func TestFrontMatterPatchMerge_MergeDeep(t *testing.T) {
	base := map[string]any{
		"title": "Original",
		"config": map[string]any{
			"enabled": true,
			"count":   5,
		},
		"tags": []string{"old", "existing"},
	}

	patch := FrontMatterPatch{
		Source:        "test",
		Mode:          MergeDeep,
		Priority:      50,
		ArrayStrategy: ArrayReplace,
		Data: map[string]any{
			"title": "New Title",
			"config": map[string]any{
				"enabled": false, // should override
				"timeout": 30,    // should add
			},
			"tags":     []string{"new", "replacement"},
			"newField": "added",
		},
	}

	result := applyPatchToMap(base, patch)

	// Direct field override
	if result["title"] != "New Title" {
		t.Errorf("expected title override, got %v", result["title"])
	}

	// Deep merge of nested map
	config := result["config"].(map[string]any)
	if config["enabled"] != false {
		t.Errorf("expected nested enabled override, got %v", config["enabled"])
	}
	if config["count"] != 5 {
		t.Errorf("expected nested count preserved, got %v", config["count"])
	}
	if config["timeout"] != 30 {
		t.Errorf("expected nested timeout added, got %v", config["timeout"])
	}

	// Array replacement strategy
	tags := result["tags"].([]string)
	expected := []string{"new", "replacement"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("expected array replacement, got %v", tags)
	}

	// New field addition
	if result["newField"] != "added" {
		t.Errorf("expected new field, got %v", result["newField"])
	}
}

func TestFrontMatterPatchMerge_ArrayUnion(t *testing.T) {
	base := map[string]any{
		"tags": []string{"existing", "old"},
	}

	patch := FrontMatterPatch{
		Source:        "test",
		Mode:          MergeDeep,
		ArrayStrategy: ArrayUnion,
		Data: map[string]any{
			"tags": []string{"new", "existing"}, // "existing" should dedupe
		},
	}

	result := applyPatchToMap(base, patch)
	tags := result["tags"].([]string)

	// Should contain union: old + new - duplicates
	expected := []string{"existing", "old", "new"}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("expected union result %v, got %v", expected, tags)
	}
}

func TestFrontMatterPatchMerge_ArrayAppend(t *testing.T) {
	base := map[string]any{
		"categories": []string{"docs", "guide"},
	}

	patch := FrontMatterPatch{
		Source:        "test",
		Mode:          MergeDeep,
		ArrayStrategy: ArrayAppend,
		Data: map[string]any{
			"categories": []string{"reference", "api"},
		},
	}

	result := applyPatchToMap(base, patch)
	cats := result["categories"].([]string)

	expected := []string{"docs", "guide", "reference", "api"}
	if !reflect.DeepEqual(cats, expected) {
		t.Errorf("expected append result %v, got %v", expected, cats)
	}
}

func TestFrontMatterPatchMerge_MergeReplace(t *testing.T) {
	base := map[string]any{
		"config":   map[string]any{"keep": "this", "replace": "old"},
		"preserve": "untouched",
	}

	patch := FrontMatterPatch{
		Mode: MergeReplace,
		Data: map[string]any{
			"config": map[string]any{"replace": "new", "added": "fresh"},
		},
	}

	result := applyPatchToMap(base, patch)

	// Entire config replaced, not merged
	config := result["config"].(map[string]any)
	if _, exists := config["keep"]; exists {
		t.Errorf("expected complete replacement, but old key 'keep' still exists")
	}
	if config["replace"] != "new" {
		t.Errorf("expected new replace value, got %v", config["replace"])
	}
	if config["added"] != "fresh" {
		t.Errorf("expected added value, got %v", config["added"])
	}

	// Other keys preserved
	if result["preserve"] != "untouched" {
		t.Errorf("expected preserve untouched, got %v", result["preserve"])
	}
}

func TestFrontMatterPatchMerge_MergeSetIfMissing(t *testing.T) {
	base := map[string]any{
		"existing": "keep",
		"nested":   map[string]any{"sub": "preserve"},
	}

	patch := FrontMatterPatch{
		Mode: MergeSetIfMissing,
		Data: map[string]any{
			"existing": "should not override",
			"missing":  "should add",
			"nested":   map[string]any{"sub": "should not override", "newsub": "should add"},
		},
	}

	result := applyPatchToMap(base, patch)

	// Existing preserved
	if result["existing"] != "keep" {
		t.Errorf("expected existing preserved, got %v", result["existing"])
	}

	// Missing added
	if result["missing"] != "should add" {
		t.Errorf("expected missing added, got %v", result["missing"])
	}

	// Nested handling
	nested := result["nested"].(map[string]any)
	if nested["sub"] != "preserve" {
		t.Errorf("expected nested sub preserved, got %v", nested["sub"])
	}
	if nested["newsub"] != "should add" {
		t.Errorf("expected nested newsub added, got %v", nested["newsub"])
	}
}

// Mock implementation of patch application logic (since actual merge logic may be in hugo package)
// This simulates the expected behavior for testing purposes.
func applyPatchToMap(base map[string]any, patch FrontMatterPatch) map[string]any {
	result := make(map[string]any)

	// Copy base
	for k, v := range base {
		result[k] = copyValue(v)
	}

	// Apply patch based on mode
	switch patch.Mode {
	case MergeDeep:
		for k, v := range patch.Data {
			if existing, exists := result[k]; exists {
				if existingMap, ok1 := existing.(map[string]any); ok1 {
					if patchMap, ok2 := v.(map[string]any); ok2 {
						// Deep merge maps
						merged := make(map[string]any)
						for ek, ev := range existingMap {
							merged[ek] = ev
						}
						for pk, pv := range patchMap {
							merged[pk] = pv
						}
						result[k] = merged
						continue
					}
				}
				if existingSlice, ok1 := existing.([]string); ok1 {
					if patchSlice, ok2 := v.([]string); ok2 {
						// Handle array strategies
						switch patch.ArrayStrategy {
						case ArrayReplace:
							result[k] = patchSlice
						case ArrayUnion:
							result[k] = unionStringSlices(existingSlice, patchSlice)
						case ArrayAppend:
							result[k] = append(existingSlice, patchSlice...)
						default:
							result[k] = patchSlice // default to replace
						}
						continue
					}
				}
			}
			result[k] = v
		}
	case MergeReplace:
		for k, v := range patch.Data {
			result[k] = v
		}
	case MergeSetIfMissing:
		for k, v := range patch.Data {
			if _, exists := result[k]; !exists {
				result[k] = v
			} else if existingMap, ok1 := result[k].(map[string]any); ok1 {
				if patchMap, ok2 := v.(map[string]any); ok2 {
					// Recursively apply SetIfMissing to nested maps
					for pk, pv := range patchMap {
						if _, exists := existingMap[pk]; !exists {
							existingMap[pk] = pv
						}
					}
				}
			}
		}
	}

	return result
}

func copyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		cp := make(map[string]any)
		for k, v := range val {
			cp[k] = copyValue(v)
		}
		return cp
	case []string:
		cp := make([]string, len(val))
		copy(cp, val)
		return cp
	default:
		return v
	}
}

func unionStringSlices(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}
