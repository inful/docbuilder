package hugo

import (
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// attachCategoriesMenu stores the result on the generator so the
// config writer can pick it up.
func (g *Generator) attachCategoriesMenu(cm *models.CategoriesMenu) {
	g.categoriesMenu = cm
}

// computeCategoriesMenu runs the pure builder against the docs whose
// content was loaded during discovery. It is exposed for tests;
// production code uses the stage.
//
// Returns (*CategoriesMenu, nil) when the categories-menu is built,
// (nil, models.ErrCategoriesMenuSkipped) when the sidebar mode is
// not categories OR when the build has no documents (so there is
// nothing to populate a menu with), and (nil, err) on any error.
func (g *Generator) computeCategoriesMenu(ctx *models.BuildState) (*models.CategoriesMenu, error) {
	if !g.config.Hugo.Sidebar.IsCategories() {
		return nil, models.ErrCategoriesMenuSkipped
	}
	// Pass the full per-category group_by map to the read path so
	// it can scope the axis extraction. The read path emits one
	// entry per (category × axis-combo); for a doc in
	// `project: [a, b]`, that means the doc lands under both `a`
	// and `b` in the sidebar of any category that has `project` in
	// its chain. Categories NOT in the map fall back to the
	// default Repository-based grouping.
	var groupBy map[string][]string
	if g.config.Hugo.Sidebar != nil && len(g.config.Hugo.Sidebar.GroupBy) > 0 {
		groupBy = g.config.Hugo.Sidebar.GroupBy
		slog.Debug("Categories menu: configured group_by",
			slog.Int("categories", len(groupBy)),
			slog.Any("group_by", groupBy))
	} else {
		slog.Debug("Categories menu: no group_by configured; using Repository-based grouping")
	}
	items := readCategoriesMenuDocs(
		ctx.Docs.Files,
		ctx.Docs.IsSingleRepo,
		g.config.IsDaemonPublicOnlyEnabled(),
		groupBy,
	)
	if len(items) == 0 {
		// Truly empty build (no docs at all). The synthetic
		// _uncategorized bucket covers the "has docs but none
		// categorized" case; if we reach this branch the doc set
		// is empty. Emit a one-line INFO so the user knows the
		// categories menu was not built, and fall back to the
		// default sidebar.
		slog.Info("No documents discovered; using default sidebar " +
			"(set hugo.sidebar.mode: default to silence this message)")
		return nil, models.ErrCategoriesMenuSkipped
	}
	segment := "repo"
	if g.config.Hugo.Sidebar != nil && g.config.Hugo.Sidebar.ProjectSegment != "" {
		segment = g.config.Hugo.Sidebar.ProjectSegment
	}
	return buildCategoriesMenu(items, g.config.Repositories, segment, groupBy)
}
