package models

import "errors"

// ErrCategoriesMenuSkipped is returned by computeCategoriesMenu when
// the sidebar mode is not "categories". Callers (the categories-menu
// stage in particular) should treat this as a benign "do nothing"
// signal and not as a build failure. Detected with errors.Is.
var ErrCategoriesMenuSkipped = errors.New("categories menu: sidebar mode is not 'categories'")

// CategoriesMenu is the data structure produced by the categories-menu
// stage and consumed by the Hugo config writer. It is intentionally
// decoupled from the hugo package's internal types so the stage
// interface stays in models.
type CategoriesMenu struct {
	// Menus is keyed by category name (e.g., "Documentation"); each
	// value is the list of Hugo menu entries that form the menu for
	// that category.
	Menus map[string][]MenuEntry

	// SidebarEntries are the Relearn `params.sidebarmenus[]` blocks
	// that wire each category menu into the sidebar.
	SidebarEntries []SidebarEntry

	// Built is true after the stage has run successfully. The config
	// writer treats a nil or unbuilt CategoriesMenu as a no-op.
	Built bool
}

// MenuEntry is a single Relearn Hugo menu entry. Only the fields the
// categories-menu stage emits are modeled; the existing hugo.menu
// config (a separate code path) uses config.Menu instead.
type MenuEntry struct {
	// Identifier is the Hugo menu identifier. Required for parent
	// entries so child entries can reference them via Parent.
	Identifier string
	Name       string
	URL        string
	PageRef    string
	Parent     string
	Weight     int
	Title      string
}

// SidebarEntry is a single Relearn `params.sidebarmenus[]` block.
type SidebarEntry struct {
	Identifier   string
	Type         string // "menu" or "page"
	PageRef      string
	DisableTitle bool
	// Weight controls the ordering of the block within the rendered
	// sidebar. The synthetic _uncategorized category is emitted with
	// a high weight (999) so it renders last regardless of
	// alphabetical sort order. Zero (or unset) leaves ordering to
	// Relearn's defaults.
	Weight int
}
