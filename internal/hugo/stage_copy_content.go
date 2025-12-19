package hugo

import (
	"context"
	"errors"
)

func stageCopyContent(ctx context.Context, bs *BuildState) error {
	if err := bs.Generator.copyContentFilesWithState(ctx, bs.Docs.Files, bs); err != nil {
		if errors.Is(err, context.Canceled) {
			return newCanceledStageError(StageCopyContent, err)
		}
		return err
	}
	return nil
}
