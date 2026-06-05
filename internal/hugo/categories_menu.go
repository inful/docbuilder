package hugo

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// categoryDoc is the minimal projection of a DocFile that the
// categories menu builder needs. Decoupling from docs.DocFile keeps
// the builder a pure function and easy to test.
type categoryDoc struct {
	Repository string
	Title      string
	Path       string // Hugo path used for the menu entry's pageRef
	Weight     int
	// Categories holds the canonical (lowercased, trimmed) category
	// keys for this doc. Using the canonical key for grouping merges
	// "Minutes" and "minutes" into a single sidebar block.
	Categories []string
	// CategoryDisplay is the original (case-preserved, trimmed)
	// spelling of the category as the doc author wrote it. The first
	// occurrence of a given canonical key — under the stable input
	// ordering produced by readCategoriesMenuDocs — wins as the
	// rendered wrapper label. When a doc declares multiple
	// categories, CategoryDisplay matches Categories[0]; the read
	// pipeline emits one categoryDoc per declared category so the
	// pairing is always one-to-one.
	CategoryDisplay string
	// GroupFields carries the raw values of any front-matter fields
	// the user has asked to be used for sub-grouping (see
	// hugo.sidebar.group_by). The map is keyed by field name; the
	// value is the trimmed front-matter string (or "" when the field
	// is missing or empty). Nil when no grouping fields were
	// requested at read time, which keeps the cost of the common
	// (no-grouping) path at zero.
	GroupFields map[string]string
	// SiblingCategories carries the doc's own category list, keyed by
	// the canonical (lowercased, trimmed) category name. The value is
	// the first-seen original spelling of that category as the doc
	// author wrote it, so the rendered label preserves intentional
	// casings (e.g. "Team-Alpha" stays "Team-Alpha"). Used by the
	// build path to resolve a group_by axis that is itself a
	// sibling category (the self-describing rule: a value in
	// group_by.<cat> that is also a key in group_by is a sibling
	// category reference). Populated only when grouping fields are
	// requested, on the same lazy path as GroupFields.
	SiblingCategories map[string]string
}

// axisKind tags a single entry in a category's group_by chain. The
// chain is a list of front-matter field names today; the sibling-
// category extension widens the chain to also accept a canonical
// category name. The kind is precomputed at build time from the
// group_by map's own keys (the self-describing rule) so the read
// path doesn't need to know which kind each axis is.
type axisKind int

const (
	// axisField is the default kind: the axis name refers to a
	// front-matter field on the doc.
	axisField axisKind = iota
	// axisCategory is the sibling-category kind: the axis name
	// refers to a sibling category in the doc's categories: list.
	// The axis name must equal the canonical-lowercase form of a
	// key in the same group_by map.
	axisCategory
)

// axisSpec is one entry in a category's group_by chain. name is the
// canonical-lowercase axis name as it appears in the config; kind
// tells the resolver whether to look it up as a front-matter field
// or a sibling category.
type axisSpec struct {
	name string
	kind axisKind
}

// UncategorizedCategory is the synthetic category name emitted for
// docs that have no `categories:` front matter. It appears as both a
// sidebar block and a Hugo taxonomy term page so users can audit and
// clean up their categorization. Exported so tests and indexes code
// can refer to it from outside the package.
var UncategorizedCategory = config.SidebarUncategorizedCategory

// buildCategoriesMenu is the pure builder used by the categories-menu
// stage. It walks the doc set, groups docs by category, and for each
// category emits:
//
//   - A Hugo menu (one per category) with one parent entry per
//     project and one child entry per doc. Parents are non-clickable
//     headers (no URL, no pageRef) so the Relearn sidebar renders
//     them as expandable section headers under which the docs of
//     that project are listed.
//   - A sidebar entry (one per category) of type "menu" pointing at
//     the Hugo menu identifier.
//
// The projectSegment argument is reserved for future segmentations
// (forge, group). Today only "repo" is supported and any other value
// returns an error so misconfiguration fails fast and loudly.
//
// groupBy, when non-empty, opts a category into a front-matter-driven
// sub-grouping under its wrapper. The map is keyed by canonical
// (lowercased) category name; the value is the ordered list of
// front-matter field names that form the grouping path. The first
// field is level 1 (project), the second is level 2, and so on;
// there is no fixed cap, so a configured chain of N fields produces
// an N-level deep sidebar block. Categories not present in the map
// use the default Repository-based grouping (a single project level
// keyed by the doc's Repository).
//
// Empty-level rule (A1): at every level, a missing or empty
// front-matter value falls back to the doc's Repository, so a doc's
// path through the tree is always fully populated and a missing
// field is visually surfaced as a Repository-named node in the
// place the configured field would have occupied.
func buildCategoriesMenu(items []categoryDoc, repos []config.Repository, projectSegment string, groupBy map[string][]string) (*models.CategoriesMenu, error) {
	if projectSegment != "repo" {
		return nil, fmt.Errorf("unsupported sidebar project_segment %q (only \"repo\" is supported)", projectSegment)
	}
	cm := &models.CategoriesMenu{
		Menus:          map[string][]models.MenuEntry{},
		SidebarEntries: []models.SidebarEntry{},
		Built:          true,
	}
	if len(items) == 0 {
		return cm, nil
	}

	// Index repositories by name once so the label lookup is O(1).
	repoByName := make(map[string]config.Repository, len(repos))
	for i := range repos {
		repoByName[repos[i].Name] = repos[i]
	}

	// grouped[category] is the root of an N-level tree. Each
	// internal node represents one grouping level; the leaf level
	// holds the doc. We use a tree rather than a fixed two-level
	// map so the depth of the rendered sidebar adapts to the
	// configured group_by chain.
	grouped := map[string]*treeNode{}

	// categoryAxes is the set of canonical category names that
	// appear as keys in the group_by map. Any value in a chain
	// that matches one of these is a sibling-category reference
	// (the self-describing rule); everything else is a front-matter
	// field. Precomputed once per call so the per-axis dispatch
	// stays O(1) per chain entry.
	categoryAxes := make(map[string]struct{}, len(groupBy))
	for k := range groupBy {
		categoryAxes[k] = struct{}{}
	}

	for _, d := range items {
		for i, raw := range d.Categories {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			cat := strings.ToLower(raw)
			display := raw
			if i == 0 && d.CategoryDisplay != "" {
				display = strings.TrimSpace(d.CategoryDisplay)
			}
			axes := axesForCategory(cat, groupBy, categoryAxes)
			path := pathForDoc(d, axes)
			insertDoc(grouped, cat, display, path, axes, d, repoByName)
		}
	}

	// Sidebar entries are emitted in sorted category order so the YAML
	// is deterministic for golden tests.
	categoryNames := make([]string, 0, len(grouped))
	for k := range grouped {
		categoryNames = append(categoryNames, k)
	}
	sort.Strings(categoryNames)

	for _, cat := range categoryNames {
		emitCategory(cm, cat, grouped[cat])
	}

	return cm, nil
}

// treeNode is a single node in the N-level grouping tree. The root
// of each category tree is the category itself (its label is the
// category display name). Each child node represents one grouping
// level — the first level is keyed by the configured group_by field
// (or the doc's Repository when no group_by is configured for the
// category), the second by the next field in the chain, and so on.
// Leaf nodes hold the docs; an intermediate node may also hold docs
// when a doc's group_by chain ends at that level (a deeper field
// was empty and fell back to the Repository, so the path is still
// fully populated even though the chain had no real value past
// that point).
type treeNode struct {
	label string
	docs  []categoryDoc
	kids  map[string]*treeNode
}

// axesForCategory returns the typed axis chain for a single
// category. The names come from groupBy[canonicalCat]; each name is
// tagged axisCategory if it is also a key in categoryAxes
// (self-describing rule: a value that is itself a group_by key is a
// sibling category), otherwise axisField. The returned slice has the
// same length as the underlying field list; nil when the category is
// not in groupBy (the caller falls back to Repository-based grouping
// with a single-entry chain).
func axesForCategory(canonicalCat string, groupBy map[string][]string, categoryAxes map[string]struct{}) []axisSpec {
	fields, ok := groupBy[canonicalCat]
	if !ok || len(fields) == 0 {
		return nil
	}
	out := make([]axisSpec, 0, len(fields))
	for _, f := range fields {
		kind := axisField
		if _, isCat := categoryAxes[f]; isCat {
			kind = axisCategory
		}
		out = append(out, axisSpec{name: f, kind: kind})
	}
	return out
}

// insertDoc places d into the category tree at grouped[cat],
// creating intermediate nodes as needed. path is the level-by-level
// key chain (length = number of configured axes for cat, or 1 for
// un-configured categories). The first-seen display label for each
// node is captured here so the emit pass doesn't need to re-derive
// it from the docs.
func insertDoc(grouped map[string]*treeNode, cat, display string, path []string, axes []axisSpec, d categoryDoc, repoByName map[string]config.Repository) {
	root, ok := grouped[cat]
	if !ok {
		root = &treeNode{
			label: categoryDisplayLabel(cat, display),
			kids:  map[string]*treeNode{},
		}
		grouped[cat] = root
	}
	node := root
	for i, key := range path {
		child, exists := node.kids[key]
		if !exists {
			child = &treeNode{
				label: labelForKey(d, key, axes, i, repoByName),
				kids:  map[string]*treeNode{},
			}
			node.kids[key] = child
		}
		node = child
	}
	node.docs = append(node.docs, d)
}

// emitCategory writes the Hugo menu entries and the sidebar entry for
// one category. The synthetic top-level wrapper entry is emitted
// first so Relearn can use its `name` as the sidebar block title;
// then each level-1 child is emitted recursively, descending one
// level in the tree per call.
func emitCategory(cm *models.CategoriesMenu, cat string, root *treeNode) {
	titleID := categoryTitleIdentifier(cat)
	cm.Menus[cat] = append(cm.Menus[cat], models.MenuEntry{
		Identifier: titleID,
		Name:       root.label,
	})
	// Emit level-1 children. Sorted for stable YAML output.
	level1Keys := make([]string, 0, len(root.kids))
	for k := range root.kids {
		level1Keys = append(level1Keys, k)
	}
	sort.Strings(level1Keys)
	for _, k1 := range level1Keys {
		emitNode(cm, cat, root.kids[k1], []string{k1}, titleID)
	}
	cm.SidebarEntries = append(cm.SidebarEntries, models.SidebarEntry{
		Identifier:   categorySidebarIdentifier(cat),
		Type:         "menu",
		DisableTitle: false,
		// The synthetic _uncategorized category renders last so it
		// does not clutter first-impression navigation. The high
		// weight overrides Relearn's default (alphabetical-by-
		// identifier) ordering.
		Weight: uncategorizedSidebarWeight(cat),
	})
}

// emitNode writes the menu entry for a single node, its docs (as
// children), and recurses into its kids. path is the level-key chain
// from the category root to this node (exclusive of the category
// itself). parentID is the menu identifier of the node's parent in
// the Hugo tree. The recursion is bounded only by the configured
// group_by chain for the category.
func emitNode(cm *models.CategoriesMenu, cat string, node *treeNode, path []string, parentID string) {
	id := categoryParentIdentifier(append([]string{cat}, path...)...)
	cm.Menus[cat] = append(cm.Menus[cat], models.MenuEntry{
		Identifier: id,
		Name:       node.label,
		Parent:     parentID,
	})
	// Docs at this level, sorted by (weight, title) for stability.
	docs := append([]categoryDoc(nil), node.docs...)
	sort.SliceStable(docs, func(i, j int) bool {
		if docs[i].Weight != docs[j].Weight {
			return docs[i].Weight < docs[j].Weight
		}
		return docs[i].Title < docs[j].Title
	})
	for _, d := range docs {
		cm.Menus[cat] = append(cm.Menus[cat], models.MenuEntry{
			Name:    d.Title,
			PageRef: d.Path,
			Parent:  id,
			Weight:  d.Weight,
		})
	}
	// Recurse into the next level. append(path, k) reuses the
	// underlying array if it has capacity; that is safe here
	// because path is not consulted after the loop.
	keys := make([]string, 0, len(node.kids))
	for k := range node.kids {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		emitNode(cm, cat, node.kids[k], append(path, k), id)
	}
}

// pathForDoc returns the level-key path for d under the given
// canonical category. When the category is configured in groupBy,
// the path has one entry per configured axis; each entry is the
// canonical-lowercase value (front-matter or sibling category) or,
// if the value is missing or empty, the doc's Repository
// (A1: per-level Repository fallback). When the category is not in
// groupBy, the path is a single entry holding the doc's Repository
// — the default 2-level behavior.
func pathForDoc(d categoryDoc, axes []axisSpec) []string {
	if len(axes) == 0 {
		return []string{d.Repository}
	}
	out := make([]string, 0, len(axes))
	for _, ax := range axes {
		out = append(out, keyForAxis(d, ax))
	}
	return out
}

// keyForAxis resolves a single axis to its canonical-lowercase key
// for d. Front-matter axes look up d.GroupFields; sibling-category
// axes look up d.SiblingCategories. Either lookup falls back to
// d.Repository when the value is missing or empty (A1).
func keyForAxis(d categoryDoc, ax axisSpec) string {
	switch ax.kind {
	case axisCategory:
		if v, ok := d.SiblingCategories[ax.name]; ok {
			v = strings.TrimSpace(v)
			if v != "" {
				return strings.ToLower(v)
			}
		}
	case axisField:
		fallthrough
	default:
		if v, present := d.GroupFields[ax.name]; present {
			v = strings.TrimSpace(v)
			if v != "" {
				return strings.ToLower(v)
			}
		}
	}
	return d.Repository
}

// labelForKey returns the display label for a single level key
// derived from d. The axis at depth axisIndex is consulted to decide
// whether the key came from a front-matter field (first-seen raw
// spelling wins so intentional casings like "team_alpha" or
// "Team-Alpha" are preserved) or a sibling category (first-seen
// original spelling of the category wins, mirroring the wrapper's
// case-preservation rule). Otherwise the key is treated as a
// Repository name and the repo's DisplayName (or Name if DisplayName
// is unset) is preferred, mirroring the rule the project level has
// always used.
func labelForKey(d categoryDoc, key string, axes []axisSpec, axisIndex int, repoByName map[string]config.Repository) string {
	if axisIndex < len(axes) {
		ax := axes[axisIndex]
		var values map[string]string
		switch ax.kind {
		case axisCategory:
			values = d.SiblingCategories
		case axisField:
			fallthrough
		default:
			values = d.GroupFields
		}
		if v, ok := values[ax.name]; ok {
			v = strings.TrimSpace(v)
			if v != "" && strings.EqualFold(v, key) {
				return v
			}
		}
	}
	if repo, ok := repoByName[key]; ok {
		return repo.Label()
	}
	return key
}

// categoryParentIdentifier builds a stable, collision-resistant
// identifier for a menu parent at an arbitrary depth. The first
// argument is always the category; subsequent arguments are the
// level keys in order (level 1, level 2, …). Identifiers must start
// with a letter and contain only letters, digits, and underscores
// per Hugo's menu rules. The two-argument form (cat, level1)
// produces the same identifier the legacy 2-level code did, so
// existing fixtures and golden tests remain byte-identical.
func categoryParentIdentifier(parts ...string) string {
	if len(parts) == 0 {
		return "x"
	}
	var b strings.Builder
	b.WriteString("cat_")
	for i, p := range parts {
		if i > 0 {
			b.WriteByte('_')
		}
		b.WriteString(slugForIdentifier(p))
	}
	return b.String()
}

// categoryTitleIdentifier returns the identifier of the synthetic
// top-level wrapper entry that holds the category's display name.
// Relearn uses the wrapper's `name` as the sidebar block title when
// the menu has a single nameless top-level entry; see
// https://mcshelby.github.io/hugo-theme-relearn/configuration/sidebar/menus/index.html#title-for-arbitrary-menus
func categoryTitleIdentifier(category string) string {
	return "cat_" + slugForIdentifier(category) + "_top"
}

// categoryDisplayLabel returns the human-visible label rendered above
// a category's sidebar block. The synthetic _uncategorized bucket
// always renders as the literal "Uncategorized". Every user category
// renders with the first original spelling observed (already chosen
// by the caller and passed in as displaySpelling); this is more
// truthful than blanket title-casing because it preserves intentional
// casings like "iOS" or "eBPF" exactly as the author wrote them.
func categoryDisplayLabel(canonicalKey, displaySpelling string) string {
	if canonicalKey == UncategorizedCategory {
		return "Uncategorized"
	}
	if displaySpelling != "" {
		return displaySpelling
	}
	return canonicalKey
}

// categorySidebarIdentifier returns the identifier to use for a
// per-category sidebar entry. It MUST equal the key under which
// the matching Hugo menu is emitted, because the Relearn template
// looks up the menu with `index site.Menus $config.identifier`.
// Returning the category name (sluggified) directly keeps the
// sidebar identifier in sync with the menu key.
func categorySidebarIdentifier(category string) string {
	return slugForIdentifier(category)
}

// slugForIdentifier replaces any non-alphanumeric character in s with
// an underscore. The result is suitable for a Hugo menu identifier.
func slugForIdentifier(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "x"
	}
	// Hugo requires identifiers to start with a letter.
	first := out[0]
	if (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') {
		return out
	}
	return "x_" + out
}

// uncategorizedSidebarWeight returns the weight to use for the sidebar
// block of the given category. The synthetic _uncategorized category
// gets a high weight (uncategorizedSidebarWeightValue) so it renders
// last in the Relearn sidebar; user categories get zero so Relearn's
// default ordering applies.
func uncategorizedSidebarWeight(cat string) int {
	if cat == UncategorizedCategory {
		return uncategorizedSidebarWeightValue
	}
	return 0
}

// uncategorizedSidebarWeightValue is the weight used for the synthetic
// _uncategorized sidebar block. High enough to outrank any plausible
// user-configured weight, but still well within Hugo's int range.
const uncategorizedSidebarWeightValue = 999

// readCategoriesMenuDocs projects each doc into the categoryDoc shape
// consumed by buildCategoriesMenu. It reads front matter from each
// non-asset doc, loading the file content on demand if the in-memory
// f.Content is not yet populated. The categories-menu stage runs
// before the copy-content stage in the pipeline, so f.Content is
// typically empty at this point; we trigger the same lazy load that
// copyContentFiles would, then read the front matter.
//
// Title resolution prefers the front matter `title` field. When the
// front matter does not declare one, the doc filename (without
// extension) is used. This is consistent with how the rest of the
// pipeline handles docs without a title.
//
// For each doc with a non-empty `categories:` list, the function
// emits one categoryDoc per declared category. For each doc with no
// categories (or an empty list), the function emits a single
// categoryDoc bucketed under UncategorizedCategory so un-categorized
// docs surface as a triage list in the rendered sidebar.
//
// When publicOnly is true (mirrors daemon.content.public_only), the
// function applies the same gate the copy-content stage uses:
//   - Docs that are not explicitly `public: true` are dropped entirely
//     (no _uncategorized fallback either).
//   - Docs whose front matter cannot be parsed or whose content cannot
//     be loaded are dropped (fail-closed: we cannot prove they are
//     meant to be public).
//
// The function does not return an error. Unreadable files and
// malformed front matter are bucketed under the synthetic
// _uncategorized category with a debug log when publicOnly is false,
// and silently dropped when publicOnly is true. They are not stage
// failures because the rest of the pipeline tolerates them.
//
// groupBy is the per-category axis chain from hugo.sidebar.group_by.
// The map is keyed by canonical-lowercase category name; the value
// is the ordered list of axis names for that category. The read
// path uses it to scope the axis extraction: a category NOT in the
// map emits a single categoryDoc with no GroupFields (the build
// path falls back to Repository-based grouping for it), while a
// category in the map emits one categoryDoc per (axis-combo)
// value combination — a doc with `project: [a, b]` in such a
// category appears in the sidebar under both `a` and `b`. Nil
// when no grouping is requested (the zero-cost path stays at zero
// cost; we never enter the fan-out loop).
func readCategoriesMenuDocs(files []docs.DocFile, isSingleRepo bool, publicOnly bool, groupBy map[string][]string) []categoryDoc {
	out := make([]categoryDoc, 0, len(files))
	for i := range files {
		f := &files[i]
		if f.IsAsset {
			continue
		}
		// Load content on demand if not already loaded. This is the
		// same lazy-load path used by the copy-content stage; it
		// keeps the categories-menu stage self-sufficient without
		// requiring it to run after the copy-content stage.
		if len(f.Content) == 0 {
			if loadErr := f.LoadContent(); loadErr != nil {
				if extra, ok := unreadableDocFallback(f, isSingleRepo, publicOnly, loadErr); ok {
					out = append(out, extra)
				}
				continue
			}
		}
		// In public-only mode, drop any doc that does not carry the
		// explicit `public: true` flag. This mirrors the gate the
		// copy-content stage uses (see copyContentFilesPipeline),
		// preventing private docs (and any categories they would
		// otherwise contribute) from reaching the sidebar.
		if publicOnly && !isPublicMarkdown(f.Content) {
			continue
		}
		fm, _, had, _, err := frontmatterops.Read(f.Content)
		if err != nil {
			// A doc with malformed front matter must not fail the
			// build; the copy-content stage tolerates it via a
			// dedicated transform, and so do we. Bucket the doc
			// under the synthetic _uncategorized category and
			// log a debug message so the user can find the offender
			// without a build failure.
			slog.Debug("Categories menu: skipping doc with malformed front matter",
				logfields.Path(f.Path), logfields.Error(err))
			// In public-only mode, isPublicMarkdown above would have
			// already returned false for a doc whose front matter is
			// malformed, so we should never reach this branch with
			// publicOnly=true. Belt-and-braces: drop here too.
			if publicOnly {
				continue
			}
			hugoPath := f.GetHugoPath(isSingleRepo)
			hugoPath = strings.TrimPrefix(hugoPath, "content/")
			pageRef := "/" + strings.TrimSuffix(hugoPath, ".md") + "/"
			out = append(out, categoryDoc{
				Repository:      f.Repository,
				Title:           f.Name,
				Path:            pageRef,
				Weight:          0,
				Categories:      []string{UncategorizedCategory},
				CategoryDisplay: UncategorizedCategory,
			})
			continue
		}
		if !had {
			// No front matter: still bucket under synthetic so the
			// doc shows up in /categories/_uncategorized/ for the
			// user to triage.
			hugoPath := f.GetHugoPath(isSingleRepo)
			hugoPath = strings.TrimPrefix(hugoPath, "content/")
			pageRef := "/" + strings.TrimSuffix(hugoPath, ".md") + "/"
			out = append(out, categoryDoc{
				Repository:      f.Repository,
				Title:           f.Name,
				Path:            pageRef,
				Weight:          0,
				Categories:      []string{UncategorizedCategory},
				CategoryDisplay: UncategorizedCategory,
			})
			continue
		}
		cats := extractStringSlice(fm, "categories")
		title := extractString(fm, "title")
		if title == "" {
			title = f.Name
		}
		weight := extractInt(fm, "weight")
		hugoPath := f.GetHugoPath(isSingleRepo)
		// Hugo's pageRef is the URL of the page (relative to baseURL),
		// which is the content path with the "content/" prefix and
		// the ".md" suffix stripped, and a trailing slash appended.
		hugoPath = strings.TrimPrefix(hugoPath, "content/")
		pageRef := "/" + strings.TrimSuffix(hugoPath, ".md") + "/"
		if len(cats) == 0 {
			// Bucket un-categorized docs under the synthetic
			// category so they appear in the sidebar (and in
			// /categories/_uncategorized/) for the user to triage.
			cats = []string{UncategorizedCategory}
		}
		// Extract the sibling-category lookup once per doc. It maps
		// canonical-lowercase category name to the first-seen
		// original spelling, so the build path can resolve a
		// sibling-category axis regardless of where the category
		// appears in the categories: list and render the label
		// with the author's original casing. Populated only when
		// grouping is requested (the zero-cost path stays at zero
		// cost; we never enter the fan-out loop).
		var siblings map[string]string
		if len(groupBy) > 0 {
			siblings = make(map[string]string, len(cats))
			for _, sib := range cats {
				display := strings.TrimSpace(sib)
				if display == "" {
					continue
				}
				key := strings.ToLower(display)
				if _, seen := siblings[key]; !seen {
					siblings[key] = display
				}
			}
		}
		// Emit one categoryDoc per declared (or synthetic) category.
		// For categories present in groupBy, the cross-product over
		// their axis values drives the fan-out: a doc with
		// `project: [a, b]` in a category configured with
		// `group_by: <cat>: [project]` appears in the sidebar
		// under both `a` and `b`. For categories NOT in groupBy, a
		// single entry is emitted with no GroupFields (the build
		// path falls back to Repository-based grouping). The
		// canonical key (lowercased + trimmed) is what we group
		// on; the original spelling rides along in CategoryDisplay
		// so the first-seen spelling can drive the sidebar label.
		for _, cat := range cats {
			display := strings.TrimSpace(cat)
			if display == "" {
				continue
			}
			key := strings.ToLower(display)
			fields := groupBy[key] // per-category scoping
			if len(fields) == 0 {
				// Un-scoped category: single entry, no GroupFields.
				out = append(out, categoryDoc{
					Repository:        f.Repository,
					Title:             title,
					Path:              pageRef,
					Weight:            weight,
					Categories:        []string{key},
					CategoryDisplay:   display,
					SiblingCategories: siblings,
				})
				continue
			}
			av := extractAxisValues(fm, fields)
			for _, combo := range axisCombinations(av) {
				out = append(out, categoryDoc{
					Repository:        f.Repository,
					Title:             title,
					Path:              pageRef,
					Weight:            weight,
					Categories:        []string{key},
					CategoryDisplay:   display,
					GroupFields:       comboToGroupFields(av, combo),
					SiblingCategories: siblings,
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Repository != out[j].Repository {
			return out[i].Repository < out[j].Repository
		}
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// unreadableDocFallback decides what to emit for a doc whose Content
// could not be loaded. In public-only mode we cannot prove the doc is
// meant to be public, so we drop it entirely (returns false). With
// public_only off, we project the doc onto the synthetic
// _uncategorized bucket so it still surfaces somewhere for the user
// to triage. A doc with no Path cannot be referenced from Hugo and is
// also dropped.
func unreadableDocFallback(f *docs.DocFile, isSingleRepo, publicOnly bool, loadErr error) (categoryDoc, bool) {
	slog.Debug("Skipping doc in categories menu: cannot read content",
		logfields.Path(f.Path), logfields.Error(loadErr))
	if publicOnly || f.Path == "" {
		return categoryDoc{}, false
	}
	hugoPath := f.GetHugoPath(isSingleRepo)
	hugoPath = strings.TrimPrefix(hugoPath, "content/")
	pageRef := "/" + strings.TrimSuffix(hugoPath, ".md") + "/"
	return categoryDoc{
		Repository:      f.Repository,
		Title:           f.Name,
		Path:            pageRef,
		Weight:          0,
		Categories:      []string{UncategorizedCategory},
		CategoryDisplay: UncategorizedCategory,
	}, true
}

func extractString(fm map[string]any, key string) string {
	if v, ok := fm[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func extractInt(fm map[string]any, key string) int {
	v, ok := fm[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func extractStringSlice(fm map[string]any, key string) []string {
	v, ok := fm[key]
	if !ok {
		return nil
	}
	switch xs := v.(type) {
	case []string:
		out := make([]string, 0, len(xs))
		for _, s := range xs {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(xs))
		for _, e := range xs {
			if s, ok := e.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	}
	return nil
}

// axisValues is the per-doc list of values for a single grouping
// axis. Always non-nil; always has at least one entry. A scalar
// front-matter field produces a one-element values list; a list
// field produces one entry per value (with empty/whitespace strings
// filtered out); a missing or empty field produces [""] (a single
// empty entry, which triggers the Repository fallback in the build
// path per the A1 rule).
type axisValues struct {
	name   string
	values []string
}

// extractAxisValues returns the per-doc values for each requested
// axis, in the same order as names. Front-matter keys are matched
// case-insensitively. Scalar and list values are both supported; a
// scalar is normalised to a single-element values list so the
// cross-product loop below has a uniform shape.
func extractAxisValues(fm map[string]any, names []string) []axisValues {
	lcFM := make(map[string][]string, len(fm))
	for k, v := range fm {
		lk := strings.ToLower(k)
		switch x := v.(type) {
		case string:
			if t := strings.TrimSpace(x); t != "" {
				lcFM[lk] = []string{t}
			}
		case []string:
			out := make([]string, 0, len(x))
			for _, s := range x {
				if t := strings.TrimSpace(s); t != "" {
					out = append(out, t)
				}
			}
			if len(out) > 0 {
				lcFM[lk] = out
			}
		case []any:
			out := make([]string, 0, len(x))
			for _, e := range x {
				if s, ok := e.(string); ok {
					if t := strings.TrimSpace(s); t != "" {
						out = append(out, t)
					}
				}
			}
			if len(out) > 0 {
				lcFM[lk] = out
			}
		}
	}
	out := make([]axisValues, 0, len(names))
	for _, name := range names {
		if vs, ok := lcFM[name]; ok {
			out = append(out, axisValues{name: name, values: vs})
		} else {
			out = append(out, axisValues{name: name, values: []string{""}})
		}
	}
	return out
}

// axisCombinations returns every index combination across the
// per-axis values lists, in lexicographic (odometer) order. The
// returned slice is reusable across calls: each element aliases
// the same backing array, so callers must copy if they need to
// retain a particular combination past the next call. The read
// path consumes each combination immediately and copies the values
// into a fresh GroupFields map, so reuse is safe there.
func axisCombinations(axes []axisValues) [][]int {
	if len(axes) == 0 {
		return [][]int{nil}
	}
	count := 1
	for _, ax := range axes {
		count *= len(ax.values)
	}
	out := make([][]int, count)
	indices := make([]int, len(axes))
	for i := range count {
		combo := make([]int, len(axes))
		copy(combo, indices)
		out[i] = combo
		// Odometer increment from the rightmost axis.
		for j := range slices.Backward(axes) {
			indices[j]++
			if indices[j] < len(axes[j].values) {
				break
			}
			indices[j] = 0
		}
	}
	return out
}

// comboToGroupFields materializes a single axis combination into a
// fresh GroupFields map. Each emitted categoryDoc gets its own
// map so two combos with the same value for an axis don't
// accidentally share state.
func comboToGroupFields(axes []axisValues, combo []int) map[string]string {
	if len(axes) == 0 {
		return nil
	}
	out := make(map[string]string, len(axes))
	for i, ax := range axes {
		out[ax.name] = ax.values[combo[i]]
	}
	return out
}
