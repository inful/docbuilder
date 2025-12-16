package hugo

import (
	"context"
	"log/slog"
	"os"
)

func stageIndexes(_ context.Context, bs *BuildState) error {
	// Skip old index generation when using new pipeline (ADR-003)
	// New pipeline already generates all indexes during content processing
	if os.Getenv("DOCBUILDER_NEW_PIPELINE") == "1" {
		slog.Debug("Skipping old index generation (using new pipeline indexes)")
		return nil
	}

	if err := bs.Generator.generateIndexPages(bs.Docs.Files); err != nil {
		return err
	}
	if bs.Report != nil && bs.Generator != nil && bs.Generator.indexTemplateUsage != nil {
		for k, v := range bs.Generator.indexTemplateUsage {
			bs.Report.IndexTemplates[k] = v
		}
	}
	return nil
}
