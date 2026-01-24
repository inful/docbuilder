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
	persistDiscoveredDocsToState(bs, docFiles)
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

func persistDiscoveredDocsToState(bs *models.BuildState, docFiles []docs.DocFile) {
	if bs.Generator == nil {
		return
	}
	sm := bs.Generator.StateManager()
	if sm == nil {
		return
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
		sm.SetRepoDocumentCount(repoURL, len(paths))
		sm.SetRepoDocFilesHash(repoURL, hash)
		if setter, ok := sm.(interface{ SetRepoDocFilePaths(string, []string) }); ok {
			setter.SetRepoDocFilePaths(repoURL, paths)
		}
	}
}
