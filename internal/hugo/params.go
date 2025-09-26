package hugo

import "fmt"
import "strings"

// addDocsyParams adds Docsy theme-specific parameters to Hugo config
func (g *Generator) addDocsyParams(params map[string]interface{}) {
    if params["version"] == nil { params["version"] = "main" }
    if params["github_repo"] == nil && len(g.config.Repositories) > 0 {
        firstRepo := g.config.Repositories[0]
        if strings.Contains(firstRepo.URL, "github.com") { params["github_repo"] = firstRepo.URL }
    }
    if params["github_branch"] == nil && len(g.config.Repositories) > 0 {
        params["github_branch"] = g.config.Repositories[0].Branch }
    if params["edit_page"] == nil { params["edit_page"] = true }
    if params["search"] == nil { params["search"] = true }
    if params["offlineSearch"] == nil { params["offlineSearch"] = true }
    if params["offlineSearchSummaryLength"] == nil { params["offlineSearchSummaryLength"] = 200 }
    if params["offlineSearchMaxResults"] == nil { params["offlineSearchMaxResults"] = 25 }
    if params["ui"] == nil { params["ui"] = map[string]interface{}{"sidebar_menu_compact": false, "sidebar_menu_foldable": true, "breadcrumb_disable": false, "taxonomy_breadcrumb_disable": false, "footer_about_disable": false, "navbar_logo": true, "navbar_translucent_over_cover_disable": false, "sidebar_search_disable": false} }
    if params["links"] == nil {
        links := map[string]interface{}{"user": []map[string]interface{}{}, "developer": []map[string]interface{}{}}
        if len(g.config.Repositories) > 0 {
            for _, repo := range g.config.Repositories {
                if strings.Contains(repo.URL, "github.com") {
                    repoLink := map[string]interface{}{ "name": fmt.Sprintf("%s Repository", titleCase(repo.Name)), "url": repo.URL, "icon": "fab fa-github", "desc": fmt.Sprintf("Development happens here for %s", repo.Name) }
                    if developerLinks, ok := links["developer"].([]map[string]interface{}); ok { links["developer"] = append(developerLinks, repoLink) }
                }
            }
        }
        params["links"] = links
    }
}

// addHextraParams adds Hextra theme-specific parameters to Hugo config
func (g *Generator) addHextraParams(params map[string]interface{}) {
    if params["search"] == nil {
        params["search"] = map[string]interface{}{"enable": true, "type": "flexsearch", "flexsearch": map[string]interface{}{"index": "content", "tokenize": "forward", "version": "0.8.143"}}
    } else {
        if b, ok := params["search"].(bool); ok { params["search"] = map[string]interface{}{"enable": b} } else if m, ok := params["search"].(map[string]interface{}); ok {
            if _, exists := m["enable"]; !exists { m["enable"] = true }
            if _, ok := m["type"]; !ok { m["type"] = "flexsearch" }
            if _, ok := m["flexsearch"]; !ok { m["flexsearch"] = map[string]interface{}{"index": "content", "tokenize": "forward", "version": "0.8.143"} } else if fm, ok := m["flexsearch"].(map[string]interface{}); ok {
                if _, ok := fm["index"]; !ok { fm["index"] = "content" }
                if _, ok := fm["tokenize"]; !ok { fm["tokenize"] = "forward" }
                if _, ok := fm["version"]; !ok { fm["version"] = "0.8.143" }
            }
        }
    }
    if params["offlineSearch"] == nil { params["offlineSearch"] = true }
    if params["offlineSearchSummaryLength"] == nil { params["offlineSearchSummaryLength"] = 200 }
    if params["offlineSearchMaxResults"] == nil { params["offlineSearchMaxResults"] = 25 }
    if _, ok := params["theme"].(map[string]interface{}); !ok { params["theme"] = map[string]interface{}{"default": "system", "displayToggle": true} }
    if params["ui"] == nil { params["ui"] = map[string]interface{}{"navbar_logo": true, "sidebar_menu_foldable": true, "sidebar_menu_compact": false, "sidebar_search_disable": false} }
    if _, ok := params["mermaid"]; !ok { params["mermaid"] = map[string]interface{}{} }
    if v, ok := params["editURL"]; !ok { params["editURL"] = map[string]interface{}{"enable": true} } else if m, ok := v.(map[string]interface{}); ok { if _, exists := m["enable"]; !exists { m["enable"] = true } }
    if _, ok := params["navbar"].(map[string]interface{}); !ok { params["navbar"] = map[string]interface{}{"width": "normal"} }
}
