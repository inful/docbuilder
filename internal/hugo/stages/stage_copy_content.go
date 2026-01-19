package stages

import (
	"context"
	"errors"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

func StageCopyContent(ctx context.Context, bs *models.BuildState) error {
	if err := bs.Generator.CopyContentFilesWithState(ctx, bs.Docs.Files, bs); err != nil {
		if errors.Is(err, context.Canceled) {
			return models.NewCanceledStageError(models.StageCopyContent, err)
		}
		return err
	}
	return nil
}
