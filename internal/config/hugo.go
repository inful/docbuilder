package config

// HugoConfig represents Hugo-specific configuration for Relearn theme
type HugoConfig struct {
	BaseURL               string            `yaml:"base_url,omitempty"`
	Title                 string            `yaml:"title"`
	Description           string            `yaml:"description,omitempty"`
	EnablePageTransitions bool              `yaml:"enable_page_transitions,omitempty"` // Enable View Transitions API for smooth page transitions
	Params                map[string]any    `yaml:"params,omitempty"`
	Menu                  map[string][]Menu `yaml:"menu,omitempty"`
	Taxonomies            map[string]string `yaml:"taxonomies,omitempty"` // custom taxonomies (e.g., "category": "categories", "tag": "tags")
	Transforms            *HugoTransforms   `yaml:"transforms,omitempty"` // optional transform filtering
}

// HugoTransforms allows users to enable/disable specific named content transforms.
// If both slices are set, Disable takes precedence over Enable.
type HugoTransforms struct {
	Enable  []string `yaml:"enable,omitempty"`  // whitelist subset (empty means all)
	Disable []string `yaml:"disable,omitempty"` // explicit deny list
}
