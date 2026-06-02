package hugo

import (
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
// (nil, models.ErrCategoriesMenuSkipped) when the sidebar mode is not
// "categories", and (nil, err) on any error.
func (g *Generator) computeCategoriesMenu(ctx *models.BuildState) (*models.CategoriesMenu, error) {
	if !g.config.Hugo.Sidebar.IsCategories() {
		return nil, models.ErrCategoriesMenuSkipped
	}
	items, err := readCategoriesMenuDocs(ctx.Docs.Files, ctx.Docs.IsSingleRepo)
	if err != nil {
		return nil, err
	}
	segment := g.config.Hugo.Sidebar.ProjectSegment
	if segment == "" {
		segment = "repo"
	}
	return buildCategoriesMenu(items, g.config.Repositories, segment)
}
