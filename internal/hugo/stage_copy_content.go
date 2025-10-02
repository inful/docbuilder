package hugo

import (
	"context"
	"errors"
)

func stageCopyContent(ctx context.Context, bs *BuildState) error {
	if err := bs.Generator.copyContentFiles(ctx, bs.Docs.Files); err != nil {
		if errors.Is(err, context.Canceled) {
			return newCanceledStageError(StageCopyContent, err)
		}
		return err
	}
	return nil
}
