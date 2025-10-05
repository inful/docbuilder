package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
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
		err = fmt.Errorf("%w: %v", build.ErrDiscovery, err)
		c.LogStageFailure(err)
		return hugo.ExecutionFailure(err)
	}

	prevCount := len(bs.Docs.Files)
	prevSet := map[string]struct{}{}
	for _, f := range bs.Docs.Files {
		prevSet[f.GetHugoPath()] = struct{}{}
	}

	bs.Docs.Files = docFiles
	bs.Docs.BuildIndexes() // Update indexes after changing files

	if prevCount > 0 {
		changed := false
		if len(docFiles) != prevCount {
			changed = true
		}
		if !changed {
			nowSet := map[string]struct{}{}
			for _, f := range docFiles {
				p := f.GetHugoPath()
				nowSet[p] = struct{}{}
				if _, ok := prevSet[p]; !ok {
					changed = true
				}
			}
			if !changed {
				for k := range prevSet {
					if _, ok := nowSet[k]; !ok {
						changed = true
						break
					}
				}
			}
		}
		if !changed && bs.Git.AllReposUnchanged {
			slog.Info("Documentation files unchanged", slog.Int("files", prevCount))
		}
	}

	repoSet := map[string]struct{}{}
	for _, f := range docFiles {
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
	for _, f := range docFiles {
		paths = append(paths, f.GetHugoPath())
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	bs.Report.DocFilesHash = hex.EncodeToString(h.Sum(nil))
}

func init() {
	// Register the discover docs command
	DefaultRegistry.Register(NewDiscoverDocsCommand())
}
