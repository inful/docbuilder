package models

import (
	"errors"
	"time"
)

// FrontMatter represents a strongly-typed Hugo front matter structure.
// This replaces the map[string]any approach with compile-time type safety.
type FrontMatter struct {
	// Core Hugo fields
	Title       string    `json:"title,omitempty" yaml:"title,omitempty"`
	Date        time.Time `json:"date,omitempty" yaml:"date,omitempty"`
	Draft       bool      `json:"draft,omitempty" yaml:"draft,omitempty"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`

	// Taxonomy fields
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Categories []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	Keywords   []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`

	// DocBuilder-specific fields
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
	Forge      string `json:"forge,omitempty" yaml:"forge,omitempty"`
	Section    string `json:"section,omitempty" yaml:"section,omitempty"`
	EditURL    string `json:"edit_url,omitempty" yaml:"edit_url,omitempty"`

	// Weight and ordering
	Weight int `json:"weight,omitempty" yaml:"weight,omitempty"`

	// Layout and rendering
	Layout string `json:"layout,omitempty" yaml:"layout,omitempty"`
	Type   string `json:"type,omitempty" yaml:"type,omitempty"`

	// Custom fields for extensibility
	// These are type-safe containers for theme-specific or custom metadata
	Custom map[string]interface{} `json:",inline" yaml:",inline"`
}

// NewFrontMatter creates a new FrontMatter with sensible defaults.
func NewFrontMatter() *FrontMatter {
	return &FrontMatter{
		Date:   time.Now(),
		Custom: make(map[string]interface{}),
	}
}

// FromMap converts a map[string]any to a strongly-typed FrontMatter.
// This provides migration path from existing untyped front matter.
func FromMap(data map[string]any) (*FrontMatter, error) {
	fm := NewFrontMatter()

	if title, ok := data["title"].(string); ok {
		fm.Title = title
	}

	if date, ok := data["date"].(time.Time); ok {
		fm.Date = date
	} else if dateStr, ok := data["date"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
			fm.Date = parsed
		} else if parsed, err := time.Parse("2006-01-02T15:04:05-07:00", dateStr); err == nil {
			fm.Date = parsed
		}
	}

	if draft, ok := data["draft"].(bool); ok {
		fm.Draft = draft
	}

	if description, ok := data["description"].(string); ok {
		fm.Description = description
	}

	if repository, ok := data["repository"].(string); ok {
		fm.Repository = repository
	}

	if forge, ok := data["forge"].(string); ok {
		fm.Forge = forge
	}

	if section, ok := data["section"].(string); ok {
		fm.Section = section
	}

	if editURL, ok := data["edit_url"].(string); ok {
		fm.EditURL = editURL
	}

	if weight, ok := data["weight"].(int); ok {
		fm.Weight = weight
	}

	if layout, ok := data["layout"].(string); ok {
		fm.Layout = layout
	}

	if typ, ok := data["type"].(string); ok {
		fm.Type = typ
	}

	// Handle taxonomy fields
	if tags, ok := data["tags"].([]interface{}); ok {
		fm.Tags = make([]string, len(tags))
		for i, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				fm.Tags[i] = tagStr
			}
		}
	} else if tags, ok := data["tags"].([]string); ok {
		fm.Tags = tags
	}

	if categories, ok := data["categories"].([]interface{}); ok {
		fm.Categories = make([]string, len(categories))
		for i, cat := range categories {
			if catStr, ok := cat.(string); ok {
				fm.Categories[i] = catStr
			}
		}
	} else if categories, ok := data["categories"].([]string); ok {
		fm.Categories = categories
	}

	if keywords, ok := data["keywords"].([]interface{}); ok {
		fm.Keywords = make([]string, len(keywords))
		for i, kw := range keywords {
			if kwStr, ok := kw.(string); ok {
				fm.Keywords[i] = kwStr
			}
		}
	} else if keywords, ok := data["keywords"].([]string); ok {
		fm.Keywords = keywords
	}

	// Store any unrecognized fields in Custom
	for key, value := range data {
		switch key {
		case "title", "date", "draft", "description", "repository", "forge",
			"section", "edit_url", "weight", "layout", "type", "tags",
			"categories", "keywords":
			// Already handled above
		default:
			fm.Custom[key] = value
		}
	}

	return fm, nil
}

// ToMap converts the strongly-typed FrontMatter to a map[string]any.
// This provides compatibility with existing code that expects maps.
func (fm *FrontMatter) ToMap() map[string]any {
	result := make(map[string]any)

	if fm.Title != "" {
		result["title"] = fm.Title
	}

	if !fm.Date.IsZero() {
		result["date"] = fm.Date.Format("2006-01-02T15:04:05-07:00")
	}

	if fm.Draft {
		result["draft"] = fm.Draft
	}

	if fm.Description != "" {
		result["description"] = fm.Description
	}

	if fm.Repository != "" {
		result["repository"] = fm.Repository
	}

	if fm.Forge != "" {
		result["forge"] = fm.Forge
	}

	if fm.Section != "" {
		result["section"] = fm.Section
	}

	if fm.EditURL != "" {
		result["edit_url"] = fm.EditURL
	}

	if fm.Weight != 0 {
		result["weight"] = fm.Weight
	}

	if fm.Layout != "" {
		result["layout"] = fm.Layout
	}

	if fm.Type != "" {
		result["type"] = fm.Type
	}

	if len(fm.Tags) > 0 {
		result["tags"] = fm.Tags
	}

	if len(fm.Categories) > 0 {
		result["categories"] = fm.Categories
	}

	if len(fm.Keywords) > 0 {
		result["keywords"] = fm.Keywords
	}

	// Add custom fields
	for key, value := range fm.Custom {
		result[key] = value
	}

	return result
}

// Clone creates a deep copy of the FrontMatter.
func (fm *FrontMatter) Clone() *FrontMatter {
	clone := &FrontMatter{
		Title:       fm.Title,
		Date:        fm.Date,
		Draft:       fm.Draft,
		Description: fm.Description,
		Repository:  fm.Repository,
		Forge:       fm.Forge,
		Section:     fm.Section,
		EditURL:     fm.EditURL,
		Weight:      fm.Weight,
		Layout:      fm.Layout,
		Type:        fm.Type,
		Custom:      make(map[string]interface{}),
	}

	// Deep copy slices
	if len(fm.Tags) > 0 {
		clone.Tags = make([]string, len(fm.Tags))
		copy(clone.Tags, fm.Tags)
	}

	if len(fm.Categories) > 0 {
		clone.Categories = make([]string, len(fm.Categories))
		copy(clone.Categories, fm.Categories)
	}

	if len(fm.Keywords) > 0 {
		clone.Keywords = make([]string, len(fm.Keywords))
		copy(clone.Keywords, fm.Keywords)
	}

	// Deep copy custom fields (shallow copy for now - could be improved)
	for key, value := range fm.Custom {
		clone.Custom[key] = value
	}

	return clone
}

// SetCustom safely sets a custom field.
func (fm *FrontMatter) SetCustom(key string, value interface{}) {
	if fm.Custom == nil {
		fm.Custom = make(map[string]interface{})
	}
	fm.Custom[key] = value
}

// GetCustom safely retrieves a custom field.
func (fm *FrontMatter) GetCustom(key string) (interface{}, bool) {
	if fm.Custom == nil {
		return nil, false
	}
	value, exists := fm.Custom[key]
	return value, exists
}

// GetCustomString retrieves a custom field as a string.
func (fm *FrontMatter) GetCustomString(key string) (string, bool) {
	value, exists := fm.GetCustom(key)
	if !exists {
		return "", false
	}
	if str, ok := value.(string); ok {
		return str, true
	}
	return "", false
}

// GetCustomInt retrieves a custom field as an integer.
func (fm *FrontMatter) GetCustomInt(key string) (int, bool) {
	value, exists := fm.GetCustom(key)
	if !exists {
		return 0, false
	}
	if i, ok := value.(int); ok {
		return i, true
	}
	return 0, false
}

// AddTag adds a tag if it doesn't already exist.
func (fm *FrontMatter) AddTag(tag string) {
	for _, existing := range fm.Tags {
		if existing == tag {
			return
		}
	}
	fm.Tags = append(fm.Tags, tag)
}

// AddCategory adds a category if it doesn't already exist.
func (fm *FrontMatter) AddCategory(category string) {
	for _, existing := range fm.Categories {
		if existing == category {
			return
		}
	}
	fm.Categories = append(fm.Categories, category)
}

// AddKeyword adds a keyword if it doesn't already exist.
func (fm *FrontMatter) AddKeyword(keyword string) {
	for _, existing := range fm.Keywords {
		if existing == keyword {
			return
		}
	}
	fm.Keywords = append(fm.Keywords, keyword)
}

// Validate performs basic validation of the front matter.
func (fm *FrontMatter) Validate() error {
	if fm.Title == "" {
		return errors.New("title is required")
	}

	if fm.Date.IsZero() {
		return errors.New("date is required")
	}

	// Repository should be set for DocBuilder-generated content
	if fm.Repository == "" {
		return errors.New("repository is required for DocBuilder content")
	}

	return nil
}
