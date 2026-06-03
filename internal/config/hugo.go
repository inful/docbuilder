package config

import "fmt"

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
//
// The default mode (when Mode is unset or empty) is "auto": when the
// build produces any documents, DocBuilder emits the categories
// sidebar. The categories sidebar also includes a synthetic
// "_uncategorized" bucket for any doc that has no `categories:` front
// matter, so un-categorized docs never disappear from navigation. To
// revert to the legacy Relearn single page menu, set Mode to "default".
type SidebarConfig struct {
	// Mode selects the sidebar wiring strategy.
	//   "" or "auto"     - emit the categories sidebar (new default).
	//   "categories"     - same as "" or "auto"; explicit opt-in.
	//   "default"        - emit Relearn's default single page menu.
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

	// GroupBy opts a category into a sub-grouping under its wrapper.
	// The map is keyed by the canonical (lowercased) category name;
	// each value is the ordered list of front-matter field names that
	// define the sub-level axis. Only the first field in the list is
	// honored today; subsequent entries are reserved for future use.
	//
	// For a doc in a configured category, the project-level key
	// becomes the value of the first non-empty configured field. When
	// none of the configured fields are set, the doc falls back to
	// its source Repository so it still surfaces under a sensible
	// heading. Categories without an entry in this map use the
	// default Repository-based grouping.
	//
	// Example:
	//   sidebar:
	//     group_by:
	//       minutes: [project]   # group "Minutes" docs by their `project:` field
	GroupBy map[string][]string `yaml:"group_by,omitempty"`
}

// SidebarMode is a typed identifier for the sidebar wiring mode.
type SidebarMode string

const (
	// SidebarModeAuto is the new default mode. It emits the categories
	// sidebar whenever the build has any documents (categorized or
	// not). An empty or unset Mode is treated as SidebarModeAuto.
	SidebarModeAuto SidebarMode = "auto"
	// SidebarModeDefault keeps the Relearn default single page menu
	// and suppresses the categories sidebar. Use this to opt out of
	// the new behavior.
	SidebarModeDefault SidebarMode = "default"
	// SidebarModeCategories is an explicit alias for SidebarModeAuto
	// (the new default). Provided for users who want to be explicit.
	SidebarModeCategories SidebarMode = "categories"
)

// SidebarUncategorizedCategory is the synthetic category name emitted
// for docs that have no `categories:` front matter. It appears as
// both a sidebar block and a Hugo taxonomy term page so users can
// audit and clean up their categorization. The leading underscore
// keeps it deterministic in sort order, and the weight:999 emission
// in the sidebar wiring pushes it to render last regardless.
const SidebarUncategorizedCategory = "_uncategorized"

// IsCategories reports whether the sidebar should be wired by categories.
// Treats the zero value (Mode unset, including the nil-config case)
// and "auto" as categories, so the new behavior is the default. The
// only way to opt out is to set Mode to "default" explicitly.
func (s *SidebarConfig) IsCategories() bool {
	if s == nil {
		return true
	}
	switch s.Mode {
	case "", SidebarModeAuto, SidebarModeCategories:
		return true
	case SidebarModeDefault:
		return false
	default:
		// Unknown mode: safe default is to NOT emit the categories
		// sidebar, so a typo in user config falls back to the
		// legacy single-page layout rather than emitting a broken
		// categories sidebar. The Normalize() call warns about
		// unknown values before this fallback is reached.
		return false
	}
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

// Normalize validates and normalizes the SidebarConfig. Unknown mode
// values are replaced with the default ("auto") and reported via the
// supplied NormalizationResult. The function is additive: it never
// fails the build for a sidebar misconfiguration, since a
// misconfigured sidebar is recoverable (a legacy single-page layout)
// and the user will see the warning in the build log.
func (s *SidebarConfig) Normalize(res *NormalizationResult) {
	if s == nil {
		return
	}
	switch s.Mode {
	case "", SidebarModeAuto, SidebarModeDefault, SidebarModeCategories:
		// Known values; nothing to normalize.
	default:
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"hugo.sidebar.mode %q is unknown; falling back to %q (the default)",
			s.Mode, SidebarModeAuto,
		))
		s.Mode = SidebarModeAuto
	}
}

// HugoTransforms allows users to enable/disable specific named content transforms.
// If both slices are set, Disable takes precedence over Enable.
type HugoTransforms struct {
	Enable  []string `yaml:"enable,omitempty"`  // whitelist subset (empty means all)
	Disable []string `yaml:"disable,omitempty"` // explicit deny list
}
