package models

import (
	"errors"
	"fmt"
	"time"
)

// MigrationHelper provides utilities for migrating from the existing
// map[string]any-based front matter system to the new strongly-typed system.
type MigrationHelper struct{}

// NewMigrationHelper creates a new migration helper.
func NewMigrationHelper() *MigrationHelper {
	return &MigrationHelper{}
}

// ConvertLegacyPatch converts a legacy map[string]any patch to a typed FrontMatterPatch.
// This is used during migration from the existing fmcore system.
func (m *MigrationHelper) ConvertLegacyPatch(legacyPatch map[string]any) (*FrontMatterPatch, error) {
	if legacyPatch == nil {
		return NewFrontMatterPatch(), nil
	}

	patch := NewFrontMatterPatch()

	// Convert known fields
	if title, ok := legacyPatch["title"].(string); ok {
		patch.SetTitle(title)
	}

	if date, ok := legacyPatch["date"].(time.Time); ok {
		patch.SetDate(date)
	} else if dateStr, ok := legacyPatch["date"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
			patch.SetDate(parsed)
		} else if parsed, err := time.Parse("2006-01-02T15:04:05-07:00", dateStr); err == nil {
			patch.SetDate(parsed)
		}
	}

	if draft, ok := legacyPatch["draft"].(bool); ok {
		patch.SetDraft(draft)
	}

	if description, ok := legacyPatch["description"].(string); ok {
		patch.SetDescription(description)
	}

	if repository, ok := legacyPatch["repository"].(string); ok {
		patch.SetRepository(repository)
	}

	if forge, ok := legacyPatch["forge"].(string); ok {
		patch.SetForge(forge)
	}

	if section, ok := legacyPatch["section"].(string); ok {
		patch.SetSection(section)
	}

	if editURL, ok := legacyPatch["edit_url"].(string); ok {
		patch.SetEditURL(editURL)
	}

	if weight, ok := legacyPatch["weight"].(int); ok {
		patch.SetWeight(weight)
	}

	if layout, ok := legacyPatch["layout"].(string); ok {
		patch.SetLayout(layout)
	}

	if typ, ok := legacyPatch["type"].(string); ok {
		patch.SetType(typ)
	}

	// Handle taxonomy fields
	if tags := m.convertToStringArray(legacyPatch["tags"]); tags != nil {
		patch.SetTags(tags)
	}

	if categories := m.convertToStringArray(legacyPatch["categories"]); categories != nil {
		patch.SetCategories(categories)
	}

	if keywords := m.convertToStringArray(legacyPatch["keywords"]); keywords != nil {
		patch.SetKeywords(keywords)
	}

	// Handle merge mode
	if modeStr, ok := legacyPatch["merge_mode"].(string); ok {
		if mode, err := m.parseMergeMode(modeStr); err == nil {
			patch.WithMergeMode(mode)
		}
	}

	// Handle array merge strategy
	if strategyStr, ok := legacyPatch["array_merge_strategy"].(string); ok {
		if strategy, err := m.parseArrayMergeStrategy(strategyStr); err == nil {
			patch.WithArrayMergeStrategy(strategy)
		}
	}

	// Store any unrecognized fields in Custom
	for key, value := range legacyPatch {
		switch key {
		case "title", "date", "draft", "description", "repository", "forge",
			"section", "edit_url", "weight", "layout", "type", "tags",
			"categories", "keywords", "merge_mode", "array_merge_strategy":
			// Already handled above
		default:
			patch.SetCustom(key, value)
		}
	}

	return patch, nil
}

// ConvertLegacyFrontMatter converts a legacy map[string]any front matter to typed FrontMatter.
func (m *MigrationHelper) ConvertLegacyFrontMatter(legacy map[string]any) (*FrontMatter, error) {
	return FromMap(legacy)
}

// CreateBasePatch creates a base front matter patch with DocBuilder-specific fields.
// This replaces the ComputeBaseFrontMatter function from the legacy system.
func (m *MigrationHelper) CreateBasePatch(title, repository, forge, section string) *FrontMatterPatch {
	patch := NewFrontMatterPatch().
		SetTitle(title).
		SetRepository(repository).
		SetDate(time.Now()).
		WithMergeMode(MergeModeSetIfMissing) // Only set if missing, allow user overrides

	if forge != "" {
		patch.SetForge(forge)
	}

	if section != "" {
		patch.SetSection(section)
	}

	return patch
}

// ApplyPatchSequence applies a sequence of patches to a front matter.
// This provides the same functionality as the legacy patch application system.
func (m *MigrationHelper) ApplyPatchSequence(base *FrontMatter, patches ...*FrontMatterPatch) (*FrontMatter, error) {
	if base == nil {
		return nil, errors.New("base front matter cannot be nil")
	}

	result := base.Clone()

	for i, patch := range patches {
		if patch == nil {
			continue
		}

		var err error
		result, err = patch.Apply(result)
		if err != nil {
			return nil, fmt.Errorf("failed to apply patch %d: %w", i, err)
		}
	}

	return result, nil
}

// ValidateFrontMatterConfig validates front matter configuration.
// This can be used to ensure migration didn't break anything.
func (m *MigrationHelper) ValidateFrontMatterConfig(fm *FrontMatter) []string {
	var warnings []string

	if fm.Title == "" {
		warnings = append(warnings, "title is empty")
	}

	if fm.Date.IsZero() {
		warnings = append(warnings, "date is not set")
	}

	if fm.Repository == "" {
		warnings = append(warnings, "repository is not set")
	}

	// Check for potentially problematic custom fields
	for key, value := range fm.Custom {
		if key == "" {
			warnings = append(warnings, "empty custom field key found")
		}

		// Warn about complex nested structures that might cause issues
		switch v := value.(type) {
		case map[string]any:
			if len(v) > 10 {
				warnings = append(warnings, fmt.Sprintf("custom field '%s' has complex nested structure", key))
			}
		case []any:
			if len(v) > 20 {
				warnings = append(warnings, fmt.Sprintf("custom field '%s' has large array", key))
			}
		}
	}

	return warnings
}

// convertToStringArray converts various array types to []string.
func (m *MigrationHelper) convertToStringArray(value any) []string {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, len(v))
		for i, item := range v {
			if str, ok := item.(string); ok {
				result[i] = str
			} else {
				result[i] = fmt.Sprintf("%v", item)
			}
		}
		return result
	case string:
		// Handle single string as array of one
		return []string{v}
	default:
		return nil
	}
}

// parseMergeMode converts a string to MergeMode.
func (m *MigrationHelper) parseMergeMode(mode string) (MergeMode, error) {
	switch mode {
	case "deep":
		return MergeModeDeep, nil
	case "replace":
		return MergeModeReplace, nil
	case "set_if_missing":
		return MergeModeSetIfMissing, nil
	default:
		return MergeModeDeep, fmt.Errorf("unknown merge mode: %s", mode)
	}
}

// parseArrayMergeStrategy converts a string to ArrayMergeStrategy.
func (m *MigrationHelper) parseArrayMergeStrategy(strategy string) (ArrayMergeStrategy, error) {
	switch strategy {
	case "append":
		return ArrayMergeStrategyAppend, nil
	case "union":
		return ArrayMergeStrategyUnion, nil
	case "replace":
		return ArrayMergeStrategyReplace, nil
	default:
		return ArrayMergeStrategyUnion, fmt.Errorf("unknown array merge strategy: %s", strategy)
	}
}
