package config

// HugoConfig represents Hugo-specific configuration for the site's theme.
type HugoConfig struct {
	BaseURL               string            `yaml:"base_url,omitempty"`
	Title                 string            `yaml:"title"`
	Description           string            `yaml:"description,omitempty"`
	EnablePageTransitions bool              `yaml:"enable_page_transitions,omitempty"` // Enable View Transitions API for smooth page transitions
	Params                map[string]any    `yaml:"params,omitempty"`
	Menu                  map[string][]Menu `yaml:"menu,omitempty"`
	Taxonomies            map[string]string `yaml:"taxonomies,omitempty"` // custom taxonomies (e.g., "category": "categories", "tag": "tags")

	// Chrome selects how the navigation chrome is rendered: "full"
	// (the default) renders it on every page, "shared" renders it on
	// the donor page only and restores it in the browser. The shared
	// mode makes the published site much smaller, and needs
	// JavaScript: the navigation appears once the page has loaded.
	Chrome string `yaml:"chrome,omitempty"`

	// CustomCSS is extra CSS inlined in the theme's head hook. When
	// set, the value is written verbatim; a whitespace-only value
	// suppresses the block entirely, so operators who want the
	// unmodified theme styling can opt out.
	CustomCSS string `yaml:"custom_css,omitempty"`
}

// ThemeDocsy is the theme DocBuilder builds with.
const ThemeDocsy = "docsy"
