package models

// RootConfig represents the top-level Hugo configuration in a typed form.
// It retains flexible maps for complex areas (params, markup, menu, outputs)
// to preserve existing YAML shape while providing compile-time fields for
// common keys.
type RootConfig struct {
	Title         string `yaml:"title"`
	Description   string `yaml:"description"`
	BaseURL       string `yaml:"baseURL"`
	LanguageCode  string `yaml:"languageCode"`
	EnableGitInfo bool   `yaml:"enableGitInfo"`

	// Flexible sections predominantly driven by theme features and user overrides
	Markup  map[string]any      `yaml:"markup,omitempty"`
	Params  map[string]any      `yaml:"params,omitempty"`
	Outputs map[string][]string `yaml:"outputs,omitempty"`
	Menu    map[string]any      `yaml:"menu,omitempty"`

	// Theme selection: use either Modules or Theme string
	Module *ModuleConfig `yaml:"module,omitempty"`
	Theme  string        `yaml:"theme,omitempty"`
}

// ModuleConfig models Hugo Modules import block.
type ModuleConfig struct {
	Imports []ModuleImport `yaml:"imports"`
}

type ModuleImport struct {
	Path string `yaml:"path"`
}

// Helpers for common Markup mutations while preserving map shape
func (rc *RootConfig) EnsureGoldmark() map[string]any {
	if rc.Markup == nil {
		rc.Markup = map[string]any{}
	}
	gm, _ := rc.Markup["goldmark"].(map[string]any)
	if gm == nil {
		gm = map[string]any{}
		rc.Markup["goldmark"] = gm
	}
	return gm
}

func (rc *RootConfig) EnsureGoldmarkRendererUnsafe() {
	gm := rc.EnsureGoldmark()
	renderer, _ := gm["renderer"].(map[string]any)
	if renderer == nil {
		renderer = map[string]any{}
		gm["renderer"] = renderer
	}
	renderer["unsafe"] = true
}

func (rc *RootConfig) EnsureGoldmarkExtensions() map[string]any {
	gm := rc.EnsureGoldmark()
	ext, _ := gm["extensions"].(map[string]any)
	if ext == nil {
		ext = map[string]any{}
		gm["extensions"] = ext
	}
	return ext
}

func (rc *RootConfig) EnableMathPassthrough() {
	ext := rc.EnsureGoldmarkExtensions()
	ext["passthrough"] = map[string]any{
		"delimiters": map[string]any{
			"block":  [][]string{{"\\[", "\\]"}, {"$$", "$$"}},
			"inline": [][]string{{"\\(", "\\)"}},
		},
		"enable": true,
	}
}

func (rc *RootConfig) EnsureHighlightDefaults() {
	if rc.Markup == nil {
		rc.Markup = map[string]any{}
	}
	hl, _ := rc.Markup["highlight"].(map[string]any)
	if hl == nil {
		hl = map[string]any{}
		rc.Markup["highlight"] = hl
	}
	if _, ok := hl["style"]; !ok {
		hl["style"] = "github"
	}
	if _, ok := hl["lineNos"]; !ok {
		hl["lineNos"] = true
	}
	if _, ok := hl["tabWidth"]; !ok {
		hl["tabWidth"] = 4
	}
	if _, ok := hl["noClasses"]; !ok {
		hl["noClasses"] = false
	}
}

// Outputs helpers
func (rc *RootConfig) EnsureOutputs() map[string][]string {
	if rc.Outputs == nil {
		rc.Outputs = make(map[string][]string)
	}
	return rc.Outputs
}

func (rc *RootConfig) SetHomeOutputsHTMLRSSJSON() {
	outs := rc.EnsureOutputs()
	outs["home"] = []string{"HTML", "RSS", "JSON"}
}
