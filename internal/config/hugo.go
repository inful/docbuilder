package config

// HugoConfig represents Hugo-specific configuration for Relearn theme.
type HugoConfig struct {
	BaseURL               string            `yaml:"base_url,omitempty"`
	Title                 string            `yaml:"title"`
	Description           string            `yaml:"description,omitempty"`
	EnablePageTransitions bool              `yaml:"enable_page_transitions,omitempty"` // Enable View Transitions API for smooth page transitions
	Params                map[string]any    `yaml:"params,omitempty"`
	Menu                  map[string][]Menu `yaml:"menu,omitempty"`
	Taxonomies            map[string]string `yaml:"taxonomies,omitempty"` // custom taxonomies (e.g., "category": "categories", "tag": "tags")
	Transforms            *HugoTransforms   `yaml:"transforms,omitempty"` // optional transform filtering
	Sidebar               *SidebarConfig    `yaml:"sidebar,omitempty"`    // sidebar configuration; nil falls back to the Relearn default page menu
}

// SidebarConfig controls how DocBuilder wires the Relearn sidebar menus.
// It is intentionally additive: when nil or Mode is empty, the existing
// default behavior (single page menu rooted at "/") is preserved.
type SidebarConfig struct {
	// Mode selects the sidebar wiring strategy.
	//   "" or "default" - emit Relearn's default single page menu (today's behavior).
	//   "categories"   - emit one Relearn Hugo menu per category, grouped by project.
	Mode SidebarMode `yaml:"mode,omitempty"`

	// KeepDefaultMenu, when true (the default), emits the original
	// main page menu in addition to the category menus. When false,
	// only the category menus are emitted.
	KeepDefaultMenu *bool `yaml:"keep_default_menu,omitempty"`

	// ProjectSegment names the segment used to group documents under
	// each category. Today only "repo" is supported; this field exists
	// so future segmentations (e.g., forge, group) can be added without
	// a schema change.
	ProjectSegment string `yaml:"project_segment,omitempty"`
}

// SidebarMode is a typed identifier for the sidebar wiring mode.
type SidebarMode string

const (
	// SidebarModeDefault keeps the Relearn default single page menu.
	SidebarModeDefault SidebarMode = "default"
	// SidebarModeCategories emits one Hugo menu per category, with
	// docs grouped under their project (repository).
	SidebarModeCategories SidebarMode = "categories"
)

// IsCategories reports whether the sidebar should be wired by categories.
func (s *SidebarConfig) IsCategories() bool {
	return s != nil && s.Mode == SidebarModeCategories
}

// ShouldKeepDefaultMenu reports whether the default main page menu
// should be emitted alongside the category menus. Defaults to true
// when the field is nil so existing layouts are preserved.
func (s *SidebarConfig) ShouldKeepDefaultMenu() bool {
	if s == nil || s.KeepDefaultMenu == nil {
		return true
	}
	return *s.KeepDefaultMenu
}

// HugoTransforms allows users to enable/disable specific named content transforms.
// If both slices are set, Disable takes precedence over Enable.
type HugoTransforms struct {
	Enable  []string `yaml:"enable,omitempty"`  // whitelist subset (empty means all)
	Disable []string `yaml:"disable,omitempty"` // explicit deny list
}
