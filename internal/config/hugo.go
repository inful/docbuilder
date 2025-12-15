package config

import "strings"

// HugoConfig represents Hugo-specific configuration
type HugoConfig struct {
	Theme             string            `yaml:"theme,omitempty"` // raw theme string from config; normalized via ThemeType()
	BaseURL           string            `yaml:"base_url,omitempty"`
	Title             string            `yaml:"title"`
	Description       string            `yaml:"description,omitempty"`
	Params            map[string]any    `yaml:"params,omitempty"`
	Menu              map[string][]Menu `yaml:"menu,omitempty"`
	Taxonomies        map[string]string `yaml:"taxonomies,omitempty"`        // custom taxonomy definitions (e.g., "tag": "tags", "category": "categories")
	Transforms        *HugoTransforms   `yaml:"transforms,omitempty"`        // optional transform filtering
	EnableTransitions bool              `yaml:"enable_page_transitions,omitempty"` // enables View Transitions API for smooth page transitions
}

// HugoTransforms allows users to enable/disable specific named content transforms.
// If both slices are set, Disable takes precedence over Enable.
type HugoTransforms struct {
	Enable  []string `yaml:"enable,omitempty"`  // whitelist subset (empty means all)
	Disable []string `yaml:"disable,omitempty"` // explicit deny list
}

// Theme is a typed enumeration of supported Hugo theme integrations.
type Theme string

// Theme constants to avoid magic strings across generator logic.
const (
	ThemeHextra Theme = "hextra"
	ThemeDocsy  Theme = "docsy"
)

// ThemeType returns the normalized typed theme value (lowercasing the raw string). Unknown themes return "".
func (h HugoConfig) ThemeType() Theme {
	s := strings.ToLower(strings.TrimSpace(h.Theme))
	switch s {
	case string(ThemeHextra):
		return ThemeHextra
	case string(ThemeDocsy):
		return ThemeDocsy
	default:
		return ""
	}
}
