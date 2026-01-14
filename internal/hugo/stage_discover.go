package hugo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sort"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

func stageDiscoverDocs(ctx context.Context, bs *BuildState) error {
	if len(bs.Git.RepoPaths) == 0 {
		return newWarnStageError(StageDiscoverDocs,
			foundationerrors.WrapError(build.ErrDiscovery, foundationerrors.CategoryValidation,
				"no repositories cloned for discovery").
				WithSeverity(foundationerrors.SeverityWarning).
				Build())
	}
	select {
	case <-ctx.Done():
		return newCanceledStageError(StageDiscoverDocs, ctx.Err())
	default:
	}
	discovery := docs.NewDiscovery(bs.Git.Repositories, &bs.Generator.config.Build)
	docFiles, err := discovery.DiscoverDocs(bs.Git.RepoPaths)
	if err != nil {
		return newFatalStageError(StageDiscoverDocs,
			foundationerrors.WrapError(err, foundationerrors.CategoryValidation,
				"document discovery failed").
				WithContext("repo_count", len(bs.Git.RepoPaths)).
				Build())
	}
	prevCount := len(bs.Docs.Files)
	prevFiles := bs.Docs.Files

	bs.Docs.Files = docFiles
	bs.Docs.IsSingleRepo = discovery.IsSingleRepo() // Set single-repo flag from discovery
	bs.Docs.BuildIndexes()                          // Update indexes after changing files

	// Detect if documentation files have changed
	if DetectDocumentChanges(prevFiles, docFiles, bs.Docs.IsSingleRepo) || !bs.Git.AllReposUnchanged {
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
	if bs.Generator != nil && bs.Generator.stateManager != nil {
		repoPaths := make(map[string][]string)
		for i := range docFiles {
			f := &docFiles[i]
			p := f.GetHugoPath(bs.Docs.IsSingleRepo)
			repoPaths[f.Repository] = append(repoPaths[f.Repository], p)
		}
		for repoName, paths := range repoPaths {
			sort.Strings(paths)
			h := sha256.New()
			for _, p := range paths {
				_, _ = h.Write([]byte(p))
				_, _ = h.Write([]byte{0})
			}
			hash := hex.EncodeToString(h.Sum(nil))
			var repoURL string
			for i := range bs.Git.Repositories {
				r := &bs.Git.Repositories[i]
				if r.Name == repoName {
					repoURL = r.URL
					break
				}
			}
			if repoURL == "" {
				repoURL = repoName
			}
			bs.Generator.stateManager.SetRepoDocumentCount(repoURL, len(paths))
			bs.Generator.stateManager.SetRepoDocFilesHash(repoURL, hash)
			if setter, ok := bs.Generator.stateManager.(interface{ SetRepoDocFilePaths(string, []string) }); ok {
				setter.SetRepoDocFilePaths(repoURL, paths)
			}
		}
	}
	if bs.Report != nil {
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
	return nil
}
