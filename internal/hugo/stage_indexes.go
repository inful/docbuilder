package hugo

import "context"

func stageIndexes(ctx context.Context, bs *BuildState) error {
	if err := bs.Generator.generateIndexPages(bs.Docs); err != nil {
		return err
	}
	if bs.Report != nil && bs.Generator != nil && bs.Generator.indexTemplateUsage != nil {
		for k, v := range bs.Generator.indexTemplateUsage {
			bs.Report.IndexTemplates[k] = v
		}
	}
	return nil
}
