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
		err = fmt.Errorf("%w: %w", build.ErrDiscovery, err)
		c.LogStageFailure(err)
		return hugo.ExecutionFailure(err)
	}

	prevCount := len(bs.Docs.Files)
	prevFiles := bs.Docs.Files

	bs.Docs.Files = docFiles
	bs.Docs.BuildIndexes() // Update indexes after changing files

	// Detect if documentation files have changed
	if detectDocumentChanges(prevFiles, docFiles) || !bs.Git.AllReposUnchanged {
		// Files or repos changed - continue with build
	} else if prevCount > 0 {
		slog.Info("Documentation files unchanged", slog.Int("files", prevCount))
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

// detectDocumentChanges checks if documentation files have changed between builds.
func detectDocumentChanges(prevFiles, newFiles []docs.DocFile) bool {
	prevCount := len(prevFiles)
	if prevCount == 0 {
		return false
	}

	// Quick count check
	if len(newFiles) != prevCount {
		return true
	}

	// Build sets for comparison
	prevSet := buildFilePathSet(prevFiles)
	nowSet := buildFilePathSet(newFiles)

	// Check for new files
	if hasNewFiles(nowSet, prevSet) {
		return true
	}

	// Check for removed files
	return hasRemovedFiles(prevSet, nowSet)
}

// buildFilePathSet creates a set of Hugo paths from doc files.
func buildFilePathSet(files []docs.DocFile) map[string]struct{} {
	set := make(map[string]struct{}, len(files))
	for _, f := range files {
		set[f.GetHugoPath()] = struct{}{}
	}
	return set
}

// hasNewFiles checks if there are any files in nowSet that aren't in prevSet.
func hasNewFiles(nowSet, prevSet map[string]struct{}) bool {
	for path := range nowSet {
		if _, exists := prevSet[path]; !exists {
			return true
		}
	}
	return false
}

// hasRemovedFiles checks if there are any files in prevSet that aren't in nowSet.
func hasRemovedFiles(prevSet, nowSet map[string]struct{}) bool {
	for path := range prevSet {
		if _, exists := nowSet[path]; !exists {
			return true
		}
	}
	return false
}
