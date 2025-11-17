package hextra

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	th "git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
)

type Theme struct{}

func (Theme) Name() config.Theme { return config.ThemeHextra }
func (Theme) Features() th.Features {
	return th.Features{
		Name: config.ThemeHextra, UsesModules: true, ModulePath: "github.com/imfing/hextra", ModuleVersion: "v0.11.0",
		EnableMathPassthrough: true, AutoMainMenu: true, SupportsPerPageEditLinks: true, ProvidesMermaidSupport: true, DefaultSearchType: "flexsearch",
	}
}
func (Theme) ApplyParams(_ th.ParamContext, params map[string]any) {
	// search config normalization
	if params["search"] == nil {
		params["search"] = map[string]any{"enable": true, "type": "flexsearch", "flexsearch": map[string]any{"index": "content", "tokenize": "forward", "version": "0.8.143"}}
	} else if b, ok := params["search"].(bool); ok {
		params["search"] = map[string]any{"enable": b}
	} else if m, ok := params["search"].(map[string]any); ok {
		if _, ok := m["enable"]; !ok {
			m["enable"] = true
		}
		if _, ok := m["type"]; !ok {
			m["type"] = "flexsearch"
		}
		if _, ok := m["flexsearch"]; !ok {
			m["flexsearch"] = map[string]any{"index": "content", "tokenize": "forward", "version": "0.8.143"}
		} else if fm, ok := m["flexsearch"].(map[string]any); ok {
			if _, ok := fm["index"]; !ok {
				fm["index"] = "content"
			}
			if _, ok := fm["tokenize"]; !ok {
				fm["tokenize"] = "forward"
			}
			if _, ok := fm["version"]; !ok {
				fm["version"] = "0.8.143"
			}
		}
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
	if _, ok := params["theme"].(map[string]any); !ok {
		params["theme"] = map[string]any{"default": "system", "displayToggle": true}
	}
	if params["ui"] == nil {
		params["ui"] = map[string]any{"navbar_logo": true, "sidebar_menu_foldable": true, "sidebar_menu_compact": false, "sidebar_search_disable": false}
	}
	if _, ok := params["mermaid"]; !ok {
		params["mermaid"] = map[string]any{}
	}
	if v, ok := params["editURL"]; !ok {
		params["editURL"] = map[string]any{"enable": true}
	} else if m, ok := v.(map[string]any); ok {
		if _, exists := m["enable"]; !exists {
			m["enable"] = true
		}
	}
	if _, ok := params["navbar"].(map[string]any); !ok {
		params["navbar"] = map[string]any{"width": "normal"}
	}
}
func (Theme) CustomizeRoot(_ th.ParamContext, _ map[string]any) {}

func init() { th.RegisterTheme(Theme{}) }
