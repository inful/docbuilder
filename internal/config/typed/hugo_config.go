package typed

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// Relearn theme constants - DocBuilder exclusively uses the Relearn theme
const (
	RelearnTheme      = "relearn"
	RelearnModulePath = "github.com/McShelby/hugo-theme-relearn"
)

// NormalizeHugoTheme always returns the Relearn theme constant
// Kept for backward compatibility with existing code
func NormalizeHugoTheme(s string) string {
	// DocBuilder only supports Relearn - normalize any input to relearn
	return RelearnTheme
}

// HugoMarkupType represents markup configuration types
type HugoMarkupType struct {
	value string
}

var (
	HugoMarkupGoldmark    = HugoMarkupType{"goldmark"}
	HugoMarkupBlackfriday = HugoMarkupType{"blackfriday"}
)

func (hm HugoMarkupType) String() string {
	return hm.value
}

// HugoConfig represents a strongly-typed Hugo configuration
type HugoConfig struct {
	// Basic Hugo settings
	Title   string                    `yaml:"title" json:"title"`
	BaseURL foundation.Option[string] `yaml:"baseURL,omitempty" json:"baseURL,omitempty"`

	// Content settings
	ContentDir string   `yaml:"contentDir,omitempty" json:"contentDir,omitempty"`
	PublishDir string   `yaml:"publishDir,omitempty" json:"publishDir,omitempty"`
	StaticDir  []string `yaml:"staticDir,omitempty" json:"staticDir,omitempty"`

	// Language and locale
	LanguageCode foundation.Option[string] `yaml:"languageCode,omitempty" json:"languageCode,omitempty"`
	TimeZone     foundation.Option[string] `yaml:"timeZone,omitempty" json:"timeZone,omitempty"`

	// Build settings
	BuildDrafts  bool `yaml:"buildDrafts" json:"buildDrafts"`
	BuildFuture  bool `yaml:"buildFuture" json:"buildFuture"`
	BuildExpired bool `yaml:"buildExpired" json:"buildExpired"`

	// Markup configuration
	MarkupType HugoMarkupType `yaml:"markup_type,omitempty" json:"markup_type,omitempty"`

	// Performance settings
	Timeout foundation.Option[time.Duration] `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Theme-specific parameters
	Params HugoParams `yaml:"params,omitempty" json:"params,omitempty"`

	// Menu configuration
	Menu MenuConfig `yaml:"menu,omitempty" json:"menu,omitempty"`

	// Module configuration (for themes that support it)
	Module foundation.Option[ModuleConfig] `yaml:"module,omitempty" json:"module,omitempty"`

	// Custom settings for advanced configurations
	CustomConfig map[string]any `yaml:"custom,omitempty" json:"custom,omitempty"`
}

// HugoParams represents strongly-typed Hugo theme parameters
type HugoParams struct {
	// Common theme parameters
	Author      foundation.Option[string] `yaml:"author,omitempty" json:"author,omitempty"`
	Description foundation.Option[string] `yaml:"description,omitempty" json:"description,omitempty"`
	Keywords    []string                  `yaml:"keywords,omitempty" json:"keywords,omitempty"`

	// Social and metadata
	Social foundation.Option[SocialConfig] `yaml:"social,omitempty" json:"social,omitempty"`

	// Edit links configuration
	EditLinks EditLinksConfig `yaml:"edit_links" json:"edit_links"`

	// Search configuration
	Search foundation.Option[SearchConfig] `yaml:"search,omitempty" json:"search,omitempty"`

	// Navigation configuration
	Navigation NavigationConfig `yaml:"navigation" json:"navigation"`

	// Custom parameters for extensibility (Relearn-specific params go here)
	Custom map[string]any `yaml:"custom,omitempty" json:"custom,omitempty"`
}

// SocialConfig represents social media configuration
type SocialConfig struct {
	GitHub   foundation.Option[string] `yaml:"github,omitempty" json:"github,omitempty"`
	Twitter  foundation.Option[string] `yaml:"twitter,omitempty" json:"twitter,omitempty"`
	LinkedIn foundation.Option[string] `yaml:"linkedin,omitempty" json:"linkedin,omitempty"`
	Email    foundation.Option[string] `yaml:"email,omitempty" json:"email,omitempty"`
}

// EditLinksConfig represents edit links configuration
type EditLinksConfig struct {
	Enabled  bool                      `yaml:"enabled" json:"enabled"`
	BaseURL  foundation.Option[string] `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	EditText foundation.Option[string] `yaml:"edit_text,omitempty" json:"edit_text,omitempty"`
	PerPage  bool                      `yaml:"per_page" json:"per_page"`
}

// SearchConfig represents search functionality configuration
type SearchConfig struct {
	Enabled   bool                      `yaml:"enabled" json:"enabled"`
	Provider  foundation.Option[string] `yaml:"provider,omitempty" json:"provider,omitempty"`
	IndexPath foundation.Option[string] `yaml:"index_path,omitempty" json:"index_path,omitempty"`
}

// NavigationConfig represents navigation configuration
type NavigationConfig struct {
	ShowTOC        bool `yaml:"show_toc" json:"show_toc"`
	TOCMaxDepth    int  `yaml:"toc_max_depth" json:"toc_max_depth"`
	ShowBreadcrumb bool `yaml:"show_breadcrumb" json:"show_breadcrumb"`
}

// MenuConfig represents Hugo menu configuration
type MenuConfig struct {
	Main   []MenuItem `yaml:"main,omitempty" json:"main,omitempty"`
	Footer []MenuItem `yaml:"footer,omitempty" json:"footer,omitempty"`
}

// MenuItem represents a strongly-typed menu item
type MenuItem struct {
	Name       string                    `yaml:"name" json:"name"`
	URL        string                    `yaml:"url" json:"url"`
	Weight     foundation.Option[int]    `yaml:"weight,omitempty" json:"weight,omitempty"`
	Identifier foundation.Option[string] `yaml:"identifier,omitempty" json:"identifier,omitempty"`
	Parent     foundation.Option[string] `yaml:"parent,omitempty" json:"parent,omitempty"`
	Pre        foundation.Option[string] `yaml:"pre,omitempty" json:"pre,omitempty"`
	Post       foundation.Option[string] `yaml:"post,omitempty" json:"post,omitempty"`
}

// ModuleConfig represents Hugo module configuration
type ModuleConfig struct {
	Imports []ModuleImport `yaml:"imports" json:"imports"`
}

// ModuleImport represents a Hugo module import
type ModuleImport struct {
	Path     string                  `yaml:"path" json:"path"`
	Disabled foundation.Option[bool] `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Mounts   []ModuleMount           `yaml:"mounts,omitempty" json:"mounts,omitempty"`
}

// ModuleMount represents a Hugo module mount
type ModuleMount struct {
	Source string `yaml:"source" json:"source"`
	Target string `yaml:"target" json:"target"`
}

// Validation methods for TypedHugoConfig

// Validate performs comprehensive validation of the Hugo configuration
func (hc *HugoConfig) Validate() foundation.ValidationResult {
	chain := foundation.NewValidatorChain(
		// Validate title is not empty
		func(config HugoConfig) foundation.ValidationResult {
			if strings.TrimSpace(config.Title) == "" {
				return foundation.Invalid(
					foundation.NewValidationError("title", "not_empty", "title cannot be empty"),
				)
			}
			return foundation.Valid()
		},

		// Validate baseURL format if provided
		func(config HugoConfig) foundation.ValidationResult {
			if config.BaseURL.IsSome() {
				baseURL := config.BaseURL.Unwrap()
				if baseURL != "" {
					if _, err := url.Parse(baseURL); err != nil {
						return foundation.Invalid(
							foundation.NewValidationError("baseURL", "valid_url",
								fmt.Sprintf("baseURL must be a valid URL: %v", err)),
						)
					}
				}
			}
			return foundation.Valid()
		},

		// Validate content directory path
		func(config HugoConfig) foundation.ValidationResult {
			if config.ContentDir != "" && !isValidPath(config.ContentDir) {
				return foundation.Invalid(
					foundation.NewValidationError("contentDir", "valid_path",
						"contentDir must be a valid relative path"),
				)
			}
			return foundation.Valid()
		},

		// Validate publish directory path
		func(config HugoConfig) foundation.ValidationResult {
			if config.PublishDir != "" && !isValidPath(config.PublishDir) {
				return foundation.Invalid(
					foundation.NewValidationError("publishDir", "valid_path",
						"publishDir must be a valid relative path"),
				)
			}
			return foundation.Valid()
		},
	)

	return chain.Validate(*hc)
}

// Helper functions

// isValidPath checks if a path is valid and safe
func isValidPath(path string) bool {
	// Basic validation - ensure it's not an absolute path and doesn't contain dangerous patterns
	if filepath.IsAbs(path) {
		return false
	}

	// Check for directory traversal attempts
	if strings.Contains(path, "..") {
		return false
	}

	// Ensure it's a clean path
	clean := filepath.Clean(path)
	return clean == path && clean != "." && clean != "/"
}
