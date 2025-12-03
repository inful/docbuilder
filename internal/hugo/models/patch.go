package models

import (
	"fmt"
	"strings"
	"time"
)

// MergeMode defines how front matter patches should be applied.
type MergeMode int

const (
	// MergeModeDeep performs deep merging of nested structures
	MergeModeDeep MergeMode = iota
	// MergeModeReplace completely replaces existing values
	MergeModeReplace
	// MergeModeSetIfMissing only sets values if they don't exist
	MergeModeSetIfMissing
)

// String returns a string representation of the MergeMode.
func (m MergeMode) String() string {
	switch m {
	case MergeModeDeep:
		return "deep"
	case MergeModeReplace:
		return "replace"
	case MergeModeSetIfMissing:
		return "set_if_missing"
	default:
		return "unknown"
	}
}

// ArrayMergeStrategy defines how arrays should be merged.
type ArrayMergeStrategy int

const (
	// ArrayMergeStrategyAppend adds new items to the end
	ArrayMergeStrategyAppend ArrayMergeStrategy = iota
	// ArrayMergeStrategyUnion merges arrays removing duplicates
	ArrayMergeStrategyUnion
	// ArrayMergeStrategyReplace completely replaces the array
	ArrayMergeStrategyReplace
)

// String returns a string representation of the ArrayMergeStrategy.
func (s ArrayMergeStrategy) String() string {
	switch s {
	case ArrayMergeStrategyAppend:
		return "append"
	case ArrayMergeStrategyUnion:
		return "union"
	case ArrayMergeStrategyReplace:
		return "replace"
	default:
		return "unknown"
	}
}

// FrontMatterPatch represents a strongly-typed patch to apply to front matter.
// This replaces the map[string]any approach with type-safe operations.
type FrontMatterPatch struct {
	// Core Hugo fields
	Title       *string    `yaml:"title,omitempty" json:"title,omitempty"`
	Date        *time.Time `yaml:"date,omitempty" json:"date,omitempty"`
	Draft       *bool      `yaml:"draft,omitempty" json:"draft,omitempty"`
	Description *string    `yaml:"description,omitempty" json:"description,omitempty"`

	// Taxonomy fields
	Tags       *[]string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Categories *[]string `yaml:"categories,omitempty" json:"categories,omitempty"`
	Keywords   *[]string `yaml:"keywords,omitempty" json:"keywords,omitempty"`

	// DocBuilder-specific fields
	Repository *string `yaml:"repository,omitempty" json:"repository,omitempty"`
	Forge      *string `yaml:"forge,omitempty" json:"forge,omitempty"`
	Section    *string `yaml:"section,omitempty" json:"section,omitempty"`
	EditURL    *string `yaml:"edit_url,omitempty" json:"edit_url,omitempty"`

	// Weight and ordering
	Weight *int `yaml:"weight,omitempty" json:"weight,omitempty"`

	// Layout and rendering
	Layout *string `yaml:"layout,omitempty" json:"layout,omitempty"`
	Type   *string `yaml:"type,omitempty" json:"type,omitempty"`

	// Custom fields for extensibility
	Custom map[string]interface{} `yaml:",inline" json:",inline"`

	// Merge configuration
	MergeMode          MergeMode          `yaml:"merge_mode,omitempty" json:"merge_mode,omitempty"`
	ArrayMergeStrategy ArrayMergeStrategy `yaml:"array_merge_strategy,omitempty" json:"array_merge_strategy,omitempty"`
}

// NewFrontMatterPatch creates a new empty patch.
func NewFrontMatterPatch() *FrontMatterPatch {
	return &FrontMatterPatch{
		Custom:             make(map[string]interface{}),
		MergeMode:          MergeModeDeep,
		ArrayMergeStrategy: ArrayMergeStrategyUnion,
	}
}

// SetTitle sets the title field in the patch.
func (p *FrontMatterPatch) SetTitle(title string) *FrontMatterPatch {
	p.Title = &title
	return p
}

// SetDate sets the date field in the patch.
func (p *FrontMatterPatch) SetDate(date time.Time) *FrontMatterPatch {
	p.Date = &date
	return p
}

// SetDraft sets the draft field in the patch.
func (p *FrontMatterPatch) SetDraft(draft bool) *FrontMatterPatch {
	p.Draft = &draft
	return p
}

// SetDescription sets the description field in the patch.
func (p *FrontMatterPatch) SetDescription(description string) *FrontMatterPatch {
	p.Description = &description
	return p
}

// SetRepository sets the repository field in the patch.
func (p *FrontMatterPatch) SetRepository(repository string) *FrontMatterPatch {
	p.Repository = &repository
	return p
}

// SetForge sets the forge field in the patch.
func (p *FrontMatterPatch) SetForge(forge string) *FrontMatterPatch {
	p.Forge = &forge
	return p
}

// SetSection sets the section field in the patch.
func (p *FrontMatterPatch) SetSection(section string) *FrontMatterPatch {
	p.Section = &section
	return p
}

// SetEditURL sets the edit URL field in the patch.
func (p *FrontMatterPatch) SetEditURL(editURL string) *FrontMatterPatch {
	p.EditURL = &editURL
	return p
}

// SetWeight sets the weight field in the patch.
func (p *FrontMatterPatch) SetWeight(weight int) *FrontMatterPatch {
	p.Weight = &weight
	return p
}

// SetLayout sets the layout field in the patch.
func (p *FrontMatterPatch) SetLayout(layout string) *FrontMatterPatch {
	p.Layout = &layout
	return p
}

// SetType sets the type field in the patch.
func (p *FrontMatterPatch) SetType(typ string) *FrontMatterPatch {
	p.Type = &typ
	return p
}

// SetTags sets the tags field in the patch.
func (p *FrontMatterPatch) SetTags(tags []string) *FrontMatterPatch {
	p.Tags = &tags
	return p
}

// SetCategories sets the categories field in the patch.
func (p *FrontMatterPatch) SetCategories(categories []string) *FrontMatterPatch {
	p.Categories = &categories
	return p
}

// SetKeywords sets the keywords field in the patch.
func (p *FrontMatterPatch) SetKeywords(keywords []string) *FrontMatterPatch {
	p.Keywords = &keywords
	return p
}

// SetCustom sets a custom field in the patch.
func (p *FrontMatterPatch) SetCustom(key string, value interface{}) *FrontMatterPatch {
	if p.Custom == nil {
		p.Custom = make(map[string]interface{})
	}
	p.Custom[key] = value
	return p
}

// WithMergeMode sets the merge mode for this patch.
func (p *FrontMatterPatch) WithMergeMode(mode MergeMode) *FrontMatterPatch {
	p.MergeMode = mode
	return p
}

// WithArrayMergeStrategy sets the array merge strategy for this patch.
func (p *FrontMatterPatch) WithArrayMergeStrategy(strategy ArrayMergeStrategy) *FrontMatterPatch {
	p.ArrayMergeStrategy = strategy
	return p
}

// Apply applies this patch to the given FrontMatter, returning a new instance.
func (p *FrontMatterPatch) Apply(fm *FrontMatter) (*FrontMatter, error) {
	if fm == nil {
		return nil, fmt.Errorf("cannot apply patch to nil front matter")
	}

	// Clone the original to avoid mutation
	result := fm.Clone()

	// Apply patch based on merge mode
	switch p.MergeMode {
	case MergeModeReplace:
		return p.applyReplace(result), nil
	case MergeModeSetIfMissing:
		return p.applySetIfMissing(result), nil
	case MergeModeDeep:
		return p.applyDeep(result), nil
	default:
		return nil, fmt.Errorf("unknown merge mode: %v", p.MergeMode)
	}
}

// applyReplace applies the patch by replacing all non-nil values.
func (p *FrontMatterPatch) applyReplace(fm *FrontMatter) *FrontMatter {
	if p.Title != nil {
		fm.Title = *p.Title
	}
	if p.Date != nil {
		fm.Date = *p.Date
	}
	if p.Draft != nil {
		fm.Draft = *p.Draft
	}
	if p.Description != nil {
		fm.Description = *p.Description
	}
	if p.Repository != nil {
		fm.Repository = *p.Repository
	}
	if p.Forge != nil {
		fm.Forge = *p.Forge
	}
	if p.Section != nil {
		fm.Section = *p.Section
	}
	if p.EditURL != nil {
		fm.EditURL = *p.EditURL
	}
	if p.Weight != nil {
		fm.Weight = *p.Weight
	}
	if p.Layout != nil {
		fm.Layout = *p.Layout
	}
	if p.Type != nil {
		fm.Type = *p.Type
	}
	if p.Tags != nil {
		fm.Tags = *p.Tags
	}
	if p.Categories != nil {
		fm.Categories = *p.Categories
	}
	if p.Keywords != nil {
		fm.Keywords = *p.Keywords
	}

	// Replace custom fields
	for key, value := range p.Custom {
		fm.SetCustom(key, value)
	}

	return fm
}

// applySetIfMissing applies the patch only for missing/empty values.
func (p *FrontMatterPatch) applySetIfMissing(fm *FrontMatter) *FrontMatter {
	if p.Title != nil && fm.Title == "" {
		fm.Title = *p.Title
	}
	if p.Date != nil && fm.Date.IsZero() {
		fm.Date = *p.Date
	}
	if p.Draft != nil && !fm.Draft {
		fm.Draft = *p.Draft
	}
	if p.Description != nil && fm.Description == "" {
		fm.Description = *p.Description
	}
	if p.Repository != nil && fm.Repository == "" {
		fm.Repository = *p.Repository
	}
	if p.Forge != nil && fm.Forge == "" {
		fm.Forge = *p.Forge
	}
	if p.Section != nil && fm.Section == "" {
		fm.Section = *p.Section
	}
	if p.EditURL != nil && fm.EditURL == "" {
		fm.EditURL = *p.EditURL
	}
	if p.Weight != nil && fm.Weight == 0 {
		fm.Weight = *p.Weight
	}
	if p.Layout != nil && fm.Layout == "" {
		fm.Layout = *p.Layout
	}
	if p.Type != nil && fm.Type == "" {
		fm.Type = *p.Type
	}
	if p.Tags != nil && len(fm.Tags) == 0 {
		fm.Tags = *p.Tags
	}
	if p.Categories != nil && len(fm.Categories) == 0 {
		fm.Categories = *p.Categories
	}
	if p.Keywords != nil && len(fm.Keywords) == 0 {
		fm.Keywords = *p.Keywords
	}

	// Set missing custom fields
	for key, value := range p.Custom {
		if _, exists := fm.GetCustom(key); !exists {
			fm.SetCustom(key, value)
		}
	}

	return fm
}

// applyDeep applies the patch with deep merging for arrays and maps.
func (p *FrontMatterPatch) applyDeep(fm *FrontMatter) *FrontMatter {
	// Apply simple fields (same as replace for non-composite types)
	if p.Title != nil {
		fm.Title = *p.Title
	}
	if p.Date != nil {
		fm.Date = *p.Date
	}
	if p.Draft != nil {
		fm.Draft = *p.Draft
	}
	if p.Description != nil {
		fm.Description = *p.Description
	}
	if p.Repository != nil {
		fm.Repository = *p.Repository
	}
	if p.Forge != nil {
		fm.Forge = *p.Forge
	}
	if p.Section != nil {
		fm.Section = *p.Section
	}
	if p.EditURL != nil {
		fm.EditURL = *p.EditURL
	}
	if p.Weight != nil {
		fm.Weight = *p.Weight
	}
	if p.Layout != nil {
		fm.Layout = *p.Layout
	}
	if p.Type != nil {
		fm.Type = *p.Type
	}

	// Apply array fields with merge strategy
	if p.Tags != nil {
		fm.Tags = p.mergeStringArray(fm.Tags, *p.Tags)
	}
	if p.Categories != nil {
		fm.Categories = p.mergeStringArray(fm.Categories, *p.Categories)
	}
	if p.Keywords != nil {
		fm.Keywords = p.mergeStringArray(fm.Keywords, *p.Keywords)
	}

	// Deep merge custom fields
	for key, value := range p.Custom {
		fm.SetCustom(key, value)
	}

	return fm
}

// mergeStringArray merges two string arrays based on the patch's array merge strategy.
func (p *FrontMatterPatch) mergeStringArray(existing, newItems []string) []string {
	switch p.ArrayMergeStrategy {
	case ArrayMergeStrategyReplace:
		return newItems
	case ArrayMergeStrategyAppend:
		return append(existing, newItems...)
	case ArrayMergeStrategyUnion:
		// Create a set to track existing items
		seen := make(map[string]bool)
		result := make([]string, 0, len(existing)+len(newItems))

		// Add existing items
		for _, item := range existing {
			if !seen[item] {
				seen[item] = true
				result = append(result, item)
			}
		}

		// Add new items that aren't duplicates
		for _, item := range newItems {
			if !seen[item] {
				seen[item] = true
				result = append(result, item)
			}
		}

		return result
	default:
		return newItems
	}
}

// ToMap converts the patch to a map[string]any for compatibility.
func (p *FrontMatterPatch) ToMap() map[string]any {
	result := make(map[string]any)

	if p.Title != nil {
		result["title"] = *p.Title
	}
	if p.Date != nil {
		result["date"] = p.Date.Format("2006-01-02T15:04:05-07:00")
	}
	if p.Draft != nil {
		result["draft"] = *p.Draft
	}
	if p.Description != nil {
		result["description"] = *p.Description
	}
	if p.Repository != nil {
		result["repository"] = *p.Repository
	}
	if p.Forge != nil {
		result["forge"] = *p.Forge
	}
	if p.Section != nil {
		result["section"] = *p.Section
	}
	if p.EditURL != nil {
		result["edit_url"] = *p.EditURL
	}
	if p.Weight != nil {
		result["weight"] = *p.Weight
	}
	if p.Layout != nil {
		result["layout"] = *p.Layout
	}
	if p.Type != nil {
		result["type"] = *p.Type
	}
	if p.Tags != nil {
		result["tags"] = *p.Tags
	}
	if p.Categories != nil {
		result["categories"] = *p.Categories
	}
	if p.Keywords != nil {
		result["keywords"] = *p.Keywords
	}

	// Add custom fields
	for key, value := range p.Custom {
		result[key] = value
	}

	return result
}

// IsEmpty returns true if the patch has no fields set.
func (p *FrontMatterPatch) IsEmpty() bool {
	return p.Title == nil &&
		p.Date == nil &&
		p.Draft == nil &&
		p.Description == nil &&
		p.Repository == nil &&
		p.Forge == nil &&
		p.Section == nil &&
		p.EditURL == nil &&
		p.Weight == nil &&
		p.Layout == nil &&
		p.Type == nil &&
		p.Tags == nil &&
		p.Categories == nil &&
		p.Keywords == nil &&
		len(p.Custom) == 0
}

// String returns a human-readable representation of the patch.
func (p *FrontMatterPatch) String() string {
	var parts []string

	if p.Title != nil {
		parts = append(parts, fmt.Sprintf("title=%q", *p.Title))
	}
	if p.Repository != nil {
		parts = append(parts, fmt.Sprintf("repository=%q", *p.Repository))
	}
	if p.Section != nil {
		parts = append(parts, fmt.Sprintf("section=%q", *p.Section))
	}
	if len(p.Custom) > 0 {
		parts = append(parts, fmt.Sprintf("custom=%d_fields", len(p.Custom)))
	}

	result := fmt.Sprintf("FrontMatterPatch{%s}", strings.Join(parts, ", "))
	return result
}
