package git

import (
	"os"
	"path/filepath"
	"strings"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	foundationerrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

func (c *Client) pruneNonDocTopLevel(repoPath string, repo appcfg.Repository) error {
	if c.buildCfg == nil || !c.buildCfg.PruneNonDocPaths {
		return nil
	}
	docRoots := map[string]struct{}{}
	for _, p := range repo.Paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimPrefix(p, "/")
		parts := strings.Split(p, "/")
		if len(parts) > 0 && parts[0] != "" {
			docRoots[parts[0]] = struct{}{}
		}
	}
	allowPatterns := c.buildCfg.PruneAllow
	denyPatterns := c.buildCfg.PruneDeny
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to read directory").WithSeverity(foundationerrors.SeverityError).WithContext("repo_path", repoPath).Build()
	}
	matchesAny := func(name string, patterns []string) bool {
		for _, pat := range patterns {
			if pat == "" {
				continue
			}
			if strings.EqualFold(pat, name) {
				return true
			}
			if ok, _ := filepath.Match(pat, name); ok {
				return true
			}
		}
		return false
	}
	for _, ent := range entries {
		name := ent.Name()
		if name == ".git" {
			continue
		}
		if _, isDoc := docRoots[name]; isDoc {
			continue
		}
		if matchesAny(name, denyPatterns) {
			if err := os.RemoveAll(filepath.Join(repoPath, name)); err != nil {
				return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to remove denied path").WithSeverity(foundationerrors.SeverityError).WithContext("path", name).Build()
			}
			continue
		}
		if matchesAny(name, allowPatterns) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(repoPath, name)); err != nil {
			return foundationerrors.WrapError(err, foundationerrors.CategoryFileSystem, "failed to remove path").WithSeverity(foundationerrors.SeverityError).WithContext("path", name).Build()
		}
	}
	return nil
}
