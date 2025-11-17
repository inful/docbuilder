package docsy

import (
	"fmt"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	th "git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
)

type Theme struct{}

func (Theme) Name() config.Theme { return config.ThemeDocsy }
func (Theme) Features() th.Features {
	return th.Features{
		Name: config.ThemeDocsy, UsesModules: true, ModulePath: "github.com/google/docsy",
		EnableOfflineSearchJSON: true,
	}
}
func (Theme) ApplyParams(g th.ParamContext, params map[string]any) {
	if params["version"] == nil {
		params["version"] = "main"
	}
	if params["github_repo"] == nil && len(g.Config().Repositories) > 0 {
		first := g.Config().Repositories[0]
		if strings.Contains(first.URL, "github.com") {
			params["github_repo"] = first.URL
		}
	}
	if params["github_branch"] == nil && len(g.Config().Repositories) > 0 {
		params["github_branch"] = g.Config().Repositories[0].Branch
	}
	if params["edit_page"] == nil {
		params["edit_page"] = true
	}
	if params["search"] == nil {
		params["search"] = true
	}
	if params["offlineSearch"] == nil {
		params["offlineSearch"] = true
	}
	if params["offlineSearchSummaryLength"] == nil {
		params["offlineSearchSummaryLength"] = 200
	}
	if params["offlineSearchMaxResults"] == nil {
		params["offlineSearchMaxResults"] = 25
	}
	if params["ui"] == nil {
		params["ui"] = map[string]any{"sidebar_menu_compact": false, "sidebar_menu_foldable": true, "breadcrumb_disable": false, "taxonomy_breadcrumb_disable": false, "footer_about_disable": false, "navbar_logo": true, "navbar_translucent_over_cover_disable": false, "sidebar_search_disable": false}
	}
	if params["links"] == nil {
		links := map[string]any{"user": []map[string]any{}, "developer": []map[string]any{}}
		for _, repo := range g.Config().Repositories {
			if strings.Contains(repo.URL, "github.com") {
				link := map[string]any{"name": fmt.Sprintf("%s Repository", th.TitleCase(repo.Name)), "url": repo.URL, "icon": "fab fa-github", "desc": fmt.Sprintf("Development happens here for %s", repo.Name)}
				if dev, ok := links["developer"].([]map[string]any); ok {
					links["developer"] = append(dev, link)
				}
			}
		}
		params["links"] = links
	}
}
func (Theme) CustomizeRoot(_ th.ParamContext, _ map[string]any) {}

func init() { th.RegisterTheme(Theme{}) }
