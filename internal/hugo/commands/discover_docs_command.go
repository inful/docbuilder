package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sort"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// DiscoverDocsCommand implements the documentation discovery stage.
type DiscoverDocsCommand struct {
	BaseCommand
}

// NewDiscoverDocsCommand creates a new discover docs command.
func NewDiscoverDocsCommand() *DiscoverDocsCommand {
	return &DiscoverDocsCommand{
		BaseCommand: NewBaseCommand(CommandMetadata{
			Name:        hugo.StageDiscoverDocs,
			Description: "Discover documentation files in cloned repositories",
			Dependencies: []hugo.StageName{
				hugo.StageCloneRepos, // Must have repositories cloned first
			},
			SkipIf: func(bs *hugo.BuildState) bool {
				return len(bs.Git.RepoPaths) == 0
			},
		}),
	}
}

// Execute runs the discover docs stage.
func (c *DiscoverDocsCommand) Execute(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution {
	c.LogStageStart()

	select {
	case <-ctx.Done():
		err := ctx.Err()
		c.LogStageFailure(err)
		return hugo.ExecutionFailure(err)
	default:
	}

	discovery := docs.NewDiscovery(bs.Git.Repositories, &bs.Generator.Config().Build)
	docFiles, err := discovery.DiscoverDocs(bs.Git.RepoPaths)
	if err != nil {
		err = foundationerrors.WrapError(err, foundationerrors.CategoryValidation,
			"documentation discovery failed").
			Build()
		c.LogStageFailure(err)
		return hugo.ExecutionFailure(err)
	}

	prevCount := len(bs.Docs.Files)
	prevFiles := bs.Docs.Files

	bs.Docs.Files = docFiles
	bs.Docs.BuildIndexes() // Update indexes after changing files

	// Detect if documentation files have changed
	if hugo.DetectDocumentChanges(prevFiles, docFiles, bs.Docs.IsSingleRepo) || !bs.Git.AllReposUnchanged {
		// Files or repos changed - continue with build
	} else if prevCount > 0 {
		slog.Info("Documentation files unchanged", slog.Int("files", prevCount))
	}

	repoSet := map[string]struct{}{}
	for i := range docFiles {
		f := &docFiles[i]
		repoSet[f.Repository] = struct{}{}
	}
	bs.Report.Repositories = len(repoSet)
	bs.Report.Files = len(docFiles)

	// Update state manager with repository statistics if available
	// Note: State manager access would require Generator interface refactoring
	// Skipped in command pattern implementation for now

	// Update report with doc files hash
	if bs.Report != nil {
		c.updateReportHash(bs, docFiles)
	}

	c.LogStageSuccess()
	return hugo.ExecutionSuccess()
}

// updateReportHash updates the build report with the overall documentation files hash.
func (c *DiscoverDocsCommand) updateReportHash(bs *hugo.BuildState, docFiles []docs.DocFile) {
	paths := make([]string, 0, len(docFiles))
	for i := range docFiles {
		f := &docFiles[i]
		paths = append(paths, f.GetHugoPath(bs.Docs.IsSingleRepo))
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	bs.Report.DocFilesHash = hex.EncodeToString(h.Sum(nil))
}
