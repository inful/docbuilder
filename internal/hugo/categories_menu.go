package hugo

import (
	"fmt"
	"log/slog"
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
func buildCategoriesMenu(items []categoryDoc, repos []config.Repository, projectSegment string) (*models.CategoriesMenu, error) {
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

	// category -> project -> []doc, keyed by the canonical
	// (lowercased) category form so case variants merge into a
	// single bucket. displayByKey remembers the first original
	// spelling seen per canonical key — that becomes the wrapper
	// label rendered in the sidebar.
	type projectDocs = []categoryDoc
	type categoryGroup = map[string]projectDocs
	grouped := make(map[string]categoryGroup)
	displayByKey := make(map[string]string)
	for _, d := range items {
		for i, raw := range d.Categories {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			key := strings.ToLower(raw)
			if grouped[key] == nil {
				grouped[key] = make(categoryGroup)
			}
			grouped[key][d.Repository] = append(grouped[key][d.Repository], d)
			if _, seen := displayByKey[key]; !seen {
				// Prefer the doc's CategoryDisplay when present and
				// the iteration is at the first declared category
				// (readCategoriesMenuDocs only ever emits one
				// category per categoryDoc, so this is the common
				// path). Otherwise fall back to the raw spelling
				// from this position of d.Categories.
				display := raw
				if i == 0 && d.CategoryDisplay != "" {
					display = strings.TrimSpace(d.CategoryDisplay)
				}
				displayByKey[key] = display
			}
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
		catEntries := grouped[cat]

		// Sort project names for stable ordering.
		projectNames := make([]string, 0, len(catEntries))
		for p := range catEntries {
			projectNames = append(projectNames, p)
		}
		sort.Strings(projectNames)

		// Prepend a single nameless top-level entry whose `name`
		// Relearn renders as the sidebar block's title. This avoids
		// needing an i18n/<lang>.toml file with a `<identifier>-menuTitle`
		// key. See:
		// https://mcshelby.github.io/hugo-theme-relearn/configuration/sidebar/menus/index.html#title-for-arbitrary-menus
		titleID := categoryTitleIdentifier(cat)
		cm.Menus[cat] = append(cm.Menus[cat], models.MenuEntry{
			Identifier: titleID,
			Name:       categoryDisplayLabel(cat, displayByKey[cat]),
		})

		// First pass: emit parent entries.
		for _, projectName := range projectNames {
			repo, ok := repoByName[projectName]
			label := projectName
			if ok {
				label = repo.Label()
			}
			parentID := categoryParentIdentifier(cat, projectName)
			cm.Menus[cat] = append(cm.Menus[cat], models.MenuEntry{
				Identifier: parentID,
				Name:       label,
				Parent:     titleID,
			})
		}

		// Second pass: emit child entries grouped under parents. We
		// sort each project's docs by weight, then by title, so the
		// sidebar order is stable and predictable.
		for _, projectName := range projectNames {
			projectDocs := catEntries[projectName]
			sort.SliceStable(projectDocs, func(i, j int) bool {
				if projectDocs[i].Weight != projectDocs[j].Weight {
					return projectDocs[i].Weight < projectDocs[j].Weight
				}
				return projectDocs[i].Title < projectDocs[j].Title
			})
			parentID := categoryParentIdentifier(cat, projectName)
			for _, d := range projectDocs {
				cm.Menus[cat] = append(cm.Menus[cat], models.MenuEntry{
					Name:    d.Title,
					PageRef: d.Path,
					Parent:  parentID,
					Weight:  d.Weight,
				})
			}
		}

		cm.SidebarEntries = append(cm.SidebarEntries, models.SidebarEntry{
			Identifier:   categorySidebarIdentifier(cat),
			Type:         "menu",
			DisableTitle: false,
			// The synthetic _uncategorized category renders last so
			// it does not clutter first-impression navigation. The
			// high weight overrides Relearn's default
			// (alphabetical-by-identifier) ordering.
			Weight: uncategorizedSidebarWeight(cat),
		})
	}

	return cm, nil
}

// categoryParentIdentifier builds a stable, collision-resistant
// identifier for a category/project parent entry. Identifiers must
// start with a letter and contain only letters, digits, and
// underscores per Hugo's menu rules.
func categoryParentIdentifier(category, project string) string {
	return "cat_" + slugForIdentifier(category) + "_" + slugForIdentifier(project)
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
// The function does not return an error today. Unreadable files
// and malformed front matter are both bucketed under the synthetic
// _uncategorized category with a debug log; they are not stage
// failures because the rest of the pipeline tolerates them.
func readCategoriesMenuDocs(files []docs.DocFile, isSingleRepo bool) []categoryDoc {
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
			if err := f.LoadContent(); err != nil {
				// A doc that cannot be read is treated as having no
				// front matter; it will be bucketed under the
				// synthetic _uncategorized category if it has a
				// non-empty path, or skipped otherwise. We do not
				// fail the build for a single unreadable doc.
				slog.Debug("Skipping doc in categories menu: cannot read content",
					logfields.Path(f.Path), logfields.Error(err))
				// We still want the doc to surface under the
				// synthetic bucket when it has a discoverable path.
				// Project that onto the uncategorized bucket.
				if f.Path != "" {
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
				}
				continue
			}
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
		// Emit one categoryDoc per declared (or synthetic) category.
		// The canonical key (lowercased + trimmed) is what we group
		// on; the original spelling rides along in CategoryDisplay
		// so the first-seen spelling can drive the sidebar label.
		for _, cat := range cats {
			display := strings.TrimSpace(cat)
			if display == "" {
				continue
			}
			key := strings.ToLower(display)
			out = append(out, categoryDoc{
				Repository:      f.Repository,
				Title:           title,
				Path:            pageRef,
				Weight:          weight,
				Categories:      []string{key},
				CategoryDisplay: display,
			})
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
