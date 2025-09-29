package hugo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

func stageDiscoverDocs(ctx context.Context, bs *BuildState) error {
	if len(bs.RepoPaths) == 0 {
		return newWarnStageError(StageDiscoverDocs, fmt.Errorf("%w: no repositories cloned", build.ErrDiscovery))
	}
	select {
	case <-ctx.Done():
		return newCanceledStageError(StageDiscoverDocs, ctx.Err())
	default:
	}
	discovery := docs.NewDiscovery(bs.Repositories, &bs.Generator.config.Build)
	docFiles, err := discovery.DiscoverDocs(bs.RepoPaths)
	if err != nil {
		return newFatalStageError(StageDiscoverDocs, fmt.Errorf("%w: %v", build.ErrDiscovery, err))
	}
	prevCount := len(bs.Docs)
	prevSet := map[string]struct{}{}
	for _, f := range bs.Docs {
		prevSet[f.GetHugoPath()] = struct{}{}
	}
	bs.Docs = docFiles
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
		if !changed && bs.AllReposUnchanged {
			slog.Info("Documentation files unchanged", slog.Int("files", prevCount))
		}
	}
	repoSet := map[string]struct{}{}
	for _, f := range docFiles {
		repoSet[f.Repository] = struct{}{}
	}
	bs.Report.Repositories = len(repoSet)
	bs.Report.Files = len(docFiles)
	if bs.Generator != nil && bs.Generator.stateManager != nil {
		repoPaths := make(map[string][]string)
		for _, f := range docFiles {
			p := f.GetHugoPath()
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
			for _, r := range bs.Repositories {
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
	return nil
}
