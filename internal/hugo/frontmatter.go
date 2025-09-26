package hugo

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// FrontMatterInput bundles inputs required to build or augment front matter.
type FrontMatterInput struct {
	File     docs.DocFile
	Existing map[string]any // parsed existing front matter (may be empty)
	Config   *config.Config
	Now      time.Time
}

// BuildFrontMatter merges existing front matter with generated defaults and theme-specific additions.
// Behavior mirrors the inlined logic previously in processMarkdownFile to preserve output parity.
func BuildFrontMatter(in FrontMatterInput) map[string]any {
	fm := map[string]any{}
	for k, v := range in.Existing { // shallow copy
		fm[k] = v
	}

	// Title
	if fm["title"] == nil && in.File.Name != "index" {
		fm["title"] = strings.ReplaceAll(titleCase(in.File.Name), "-", " ")
	}
	// Date
	if fm["date"] == nil {
		fm["date"] = in.Now.Format("2006-01-02T15:04:05-07:00")
	}
	// Repository & Section
	fm["repository"] = in.File.Repository
	if in.File.Section != "" {
		fm["section"] = in.File.Section
	}
	// Metadata passthrough
	for k, v := range in.File.Metadata {
		if fm[k] == nil {
			fm[k] = v
		}
	}

	// Hextra per-page editURL generation (copied from previous inline logic)
	if in.Config != nil && in.Config.Hugo.Theme == "hextra" {
		if _, exists := fm["editURL"]; !exists { // respect existing
			hasSiteEditBase := false
			if in.Config.Hugo.Params != nil {
				if v, ok := in.Config.Hugo.Params["editURL"]; ok {
					if m, ok := v.(map[string]any); ok {
						if b, ok := m["base"].(string); ok && b != "" {
							hasSiteEditBase = true
						}
					}
				}
			}
			if !hasSiteEditBase {
				var repoCfg *config.Repository
				for i := range in.Config.Repositories {
					if in.Config.Repositories[i].Name == in.File.Repository {
						repoCfg = &in.Config.Repositories[i]
						break
					}
				}
				if repoCfg != nil {
					branch := repoCfg.Branch
					if branch == "" {
						branch = "main"
					}
					repoRel := in.File.RelativePath
					if base := strings.TrimSpace(in.File.DocsBase); base != "" && base != "." {
						repoRel = filepath.ToSlash(filepath.Join(base, repoRel))
					} else {
						repoRel = filepath.ToSlash(repoRel)
					}
					editURL := ""
					if strings.Contains(repoCfg.URL, "github.com") {
						url := strings.TrimSuffix(repoCfg.URL, ".git")
						if strings.HasPrefix(url, "git@github.com:") {
							url = strings.TrimPrefix(url, "git@github.com:")
							url = "https://github.com/" + url
						}
						editURL = fmt.Sprintf("%s/edit/%s/%s", url, branch, repoRel)
					} else if strings.Contains(repoCfg.URL, "gitlab.com") {
						url := strings.TrimSuffix(repoCfg.URL, ".git")
						if strings.HasPrefix(url, "git@gitlab.com:") {
							url = strings.TrimPrefix(url, "git@gitlab.com:")
							url = "https://gitlab.com/" + url
						}
						editURL = fmt.Sprintf("%s/-/edit/%s/%s", url, branch, repoRel)
					} else if strings.Contains(repoCfg.URL, "bitbucket.org") {
						url := strings.TrimSuffix(repoCfg.URL, ".git")
						editURL = fmt.Sprintf("%s/src/%s/%s?mode=edit", url, branch, repoRel)
					} else if strings.Contains(repoCfg.URL, "git.home.luguber.info") {
						url := strings.TrimSuffix(repoCfg.URL, ".git")
						editURL = fmt.Sprintf("%s/_edit/%s/%s", url, branch, repoRel)
					}
					if editURL != "" {
						fm["editURL"] = editURL
					}
				}
			}
		}
	}

	return fm
}

// parseExistingFrontMatter is a helper that returns existing map or empty when nil.
func parseExistingFrontMatter(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// (Future) Additional front matter transformations can compose here.
