package hugo

import (
	"fmt"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// generatePerPageEditURL returns a theme-specific edit URL for a doc file or empty string if it should not be generated.
// Conditions for generation (current behavior parity):
//   - Theme is "hextra"
//   - Site-level params.editURL.base not set
//   - Corresponding repository config is found
//   - Repository URL matches a supported forge pattern (github, gitlab, bitbucket, gitea/forgejo)
func generatePerPageEditURL(cfg *config.Config, file docs.DocFile) string {
	if cfg == nil || cfg.Hugo.Theme != "hextra" {
		return ""
	}
	if cfg.Hugo.Params != nil { // site-level base suppression
		if v, ok := cfg.Hugo.Params["editURL"]; ok {
			if m, ok := v.(map[string]any); ok {
				if b, ok := m["base"].(string); ok && b != "" {
					return ""
				}
			}
		}
	}
	// Find repo config
	var repoCfg *config.Repository
	for i := range cfg.Repositories {
		if cfg.Repositories[i].Name == file.Repository {
			repoCfg = &cfg.Repositories[i]
			break
		}
	}
	if repoCfg == nil {
		return ""
	}
	branch := repoCfg.Branch
	if branch == "" {
		branch = "main"
	}
	repoRel := file.RelativePath
	if base := strings.TrimSpace(file.DocsBase); base != "" && base != "." {
		repoRel = filepath.ToSlash(filepath.Join(base, repoRel))
	} else {
		repoRel = filepath.ToSlash(repoRel)
	}
	url := strings.TrimSuffix(repoCfg.URL, ".git")
	switch {
	case strings.Contains(repoCfg.URL, "github.com"):
		if strings.HasPrefix(url, "git@github.com:") {
			url = "https://github.com/" + strings.TrimPrefix(url, "git@github.com:")
		}
		return fmt.Sprintf("%s/edit/%s/%s", url, branch, repoRel)
	case strings.Contains(repoCfg.URL, "gitlab.com"):
		if strings.HasPrefix(url, "git@gitlab.com:") {
			url = "https://gitlab.com/" + strings.TrimPrefix(url, "git@gitlab.com:")
		}
		return fmt.Sprintf("%s/-/edit/%s/%s", url, branch, repoRel)
	case strings.Contains(repoCfg.URL, "bitbucket.org"):
		return fmt.Sprintf("%s/src/%s/%s?mode=edit", url, branch, repoRel)
	case strings.Contains(repoCfg.URL, "git.home.luguber.info"):
		return fmt.Sprintf("%s/_edit/%s/%s", url, branch, repoRel)
	}
	return ""
}
