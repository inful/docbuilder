package stages

import (
	"context"
	"errors"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// StageCategoriesMenu builds the Relearn sidebar menus for the
// categories mode. It runs after StagePrepareOutput (so the output
// directory exists) and before StageGenerateConfig (so the menu data
// is available when hugo.yaml is emitted).
//
// The stage is a no-op when hugo.sidebar.mode is not "categories".
func StageCategoriesMenu(_ context.Context, bs *models.BuildState) error {
	cm, err := bs.Generator.ComputeCategoriesMenu(bs)
	if err != nil {
		if errors.Is(err, models.ErrCategoriesMenuSkipped) {
			slog.Debug("Categories menu stage skipped (mode is not 'categories')")
			return nil
		}
		return err
	}
	bs.Generator.AttachCategoriesMenu(cm)
	slog.Info("Generated categories sidebar menus",
		slog.Int("categories", len(cm.Menus)))
	return nil
}
