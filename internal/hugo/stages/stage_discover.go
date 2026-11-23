package stages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

func StageDiscoverDocs(ctx context.Context, bs *models.BuildState) error {
	if len(bs.Git.RepoPaths) == 0 {
		return models.NewWarnStageError(models.StageDiscoverDocs, fmt.Errorf("%w: no repositories cloned", models.ErrDiscovery))
	}
	select {
	case <-ctx.Done():
		return models.NewCanceledStageError(models.StageDiscoverDocs, ctx.Err())
	default:
	}
	discovery := docs.NewDiscovery(bs.Git.Repositories, &bs.Generator.Config().Build)
	docFiles, err := discovery.DiscoverDocs(bs.Git.RepoPaths)
	if err != nil {
		return models.NewFatalStageError(models.StageDiscoverDocs, fmt.Errorf("%w: %w", models.ErrDiscovery, err))
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
	if err := persistDiscoveredDocsToState(bs, docFiles); err != nil {
		return models.NewFatalStageError(models.StageDiscoverDocs, err)
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

func persistDiscoveredDocsToState(bs *models.BuildState, docFiles []docs.DocFile) error {
	if bs.Generator == nil {
		return nil
	}
	sm := bs.Generator.StateManager()
	if sm == nil {
		return nil
	}

	repoCfgByName := make(map[string]config.Repository, len(bs.Git.Repositories))
	for i := range bs.Git.Repositories {
		r := &bs.Git.Repositories[i]
		repoCfgByName[r.Name] = *r
	}
	init, _ := sm.(interface {
		EnsureRepositoryState(url, name, branch string)
	})
	pathsByRepo := make(map[string][]string)
	for i := range docFiles {
		f := &docFiles[i]
		p := f.GetHugoPath(bs.Docs.IsSingleRepo)
		pathsByRepo[f.Repository] = append(pathsByRepo[f.Repository], p)
	}
	// The number of documents a repository had in the last build: a
	// repository that had documentation and has none now has lost it.
	previousCount, _ := sm.(interface {
		GetRepoDocumentCount(url string) int
	})
	// The repositories that have no file at all are part of the list:
	// the loop walks the files of the repositories, and a repository
	// that lost its documentation has none.
	for i := range bs.Git.Repositories {
		repo := &bs.Git.Repositories[i]
		if _, known := pathsByRepo[repo.Name]; !known {
			pathsByRepo[repo.Name] = nil
		}
	}

	for repoName, paths := range pathsByRepo {
		sort.Strings(paths)
		h := sha256.New()
		for _, p := range paths {
			_, _ = h.Write([]byte(p))
			_, _ = h.Write([]byte{0})
		}
		hash := hex.EncodeToString(h.Sum(nil))
		repoURL := repoName
		repoBranch := ""
		if cfg, ok := repoCfgByName[repoName]; ok {
			if cfg.URL != "" {
				repoURL = cfg.URL
			}
			repoBranch = cfg.Branch
		}
		if init != nil {
			init.EnsureRepositoryState(repoURL, repoName, repoBranch)
		}

		// A repository that had documentation and has none now has lost
		// it: the directory was renamed, or the files were deleted. The
		// build has to fail: the site that is published now would have
		// the pages of that repository missing, and it would replace the
		// site that still has them.
		if repositoryLostDocumentation(previousCount, repoURL, len(paths)) {
			slog.Error("Repository lost its documentation",
				slog.String("repository", repoName),
				slog.Int("documents_before", previousCount.GetRepoDocumentCount(repoURL)))
			return fmt.Errorf("the repository %s had %d documents and has none now", repoName, previousCount.GetRepoDocumentCount(repoURL))
		}

		sm.SetRepoDocumentCount(repoURL, len(paths))
		sm.SetRepoDocFilesHash(repoURL, hash)
		// The paths of the pages are not stored: the hash of the files of
		// the repository covers the change detection, and the paths made
		// the state file grow to megabytes.
	}

	return nil
}

// documentCountReader reads the number of documents a repository had in
// the last build.
type documentCountReader interface {
	GetRepoDocumentCount(url string) int
}

// repositoryLostDocumentation reports whether a repository that had
// documentation has none now.
//
// A repository that never had documentation is filtered by the
// discovery, and that is not an error: many repositories of an
// organization are not documentation.
func repositoryLostDocumentation(reader documentCountReader, repoURL string, documents int) bool {
	if documents > 0 || reader == nil {
		return false
	}
	return reader.GetRepoDocumentCount(repoURL) > 0
}
