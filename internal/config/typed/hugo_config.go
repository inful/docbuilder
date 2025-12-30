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
	Title   string                    `json:"title" yaml:"title"`
	BaseURL foundation.Option[string] `json:"baseURL,omitempty" yaml:"baseURL,omitempty"`

	// Content settings
	ContentDir string   `json:"contentDir,omitempty" yaml:"contentDir,omitempty"`
	PublishDir string   `json:"publishDir,omitempty" yaml:"publishDir,omitempty"`
	StaticDir  []string `json:"staticDir,omitempty" yaml:"staticDir,omitempty"`

	// Language and locale
	LanguageCode foundation.Option[string] `json:"languageCode,omitempty" yaml:"languageCode,omitempty"`
	TimeZone     foundation.Option[string] `json:"timeZone,omitempty" yaml:"timeZone,omitempty"`

	// Build settings
	BuildDrafts  bool `json:"buildDrafts" yaml:"buildDrafts"`
	BuildFuture  bool `json:"buildFuture" yaml:"buildFuture"`
	BuildExpired bool `json:"buildExpired" yaml:"buildExpired"`

	// Markup configuration
	MarkupType HugoMarkupType `json:"markup_type,omitempty" yaml:"markup_type,omitempty"`

	// Performance settings
	Timeout foundation.Option[time.Duration] `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	// Theme-specific parameters
	Params HugoParams `json:"params,omitempty" yaml:"params,omitempty"`

	// Menu configuration
	Menu MenuConfig `json:"menu,omitempty" yaml:"menu,omitempty"`

	// Module configuration (for themes that support it)
	Module foundation.Option[ModuleConfig] `json:"module,omitempty" yaml:"module,omitempty"`

	// Custom settings for advanced configurations
	CustomConfig map[string]any `json:"custom,omitempty" yaml:"custom,omitempty"`
}

// HugoParams represents strongly-typed Hugo theme parameters
type HugoParams struct {
	// Common theme parameters
	Author      foundation.Option[string] `json:"author,omitempty" yaml:"author,omitempty"`
	Description foundation.Option[string] `json:"description,omitempty" yaml:"description,omitempty"`
	Keywords    []string                  `json:"keywords,omitempty" yaml:"keywords,omitempty"`

	// Social and metadata
	Social foundation.Option[SocialConfig] `json:"social,omitempty" yaml:"social,omitempty"`

	// Edit links configuration
	EditLinks EditLinksConfig `json:"edit_links" yaml:"edit_links"`

	// Search configuration
	Search foundation.Option[SearchConfig] `json:"search,omitempty" yaml:"search,omitempty"`

	// Navigation configuration
	Navigation NavigationConfig `json:"navigation" yaml:"navigation"`

	// Custom parameters for extensibility (Relearn-specific params go here)
	Custom map[string]any `json:"custom,omitempty" yaml:"custom,omitempty"`
}

// SocialConfig represents social media configuration
type SocialConfig struct {
	GitHub   foundation.Option[string] `json:"github,omitempty" yaml:"github,omitempty"`
	Twitter  foundation.Option[string] `json:"twitter,omitempty" yaml:"twitter,omitempty"`
	LinkedIn foundation.Option[string] `json:"linkedin,omitempty" yaml:"linkedin,omitempty"`
	Email    foundation.Option[string] `json:"email,omitempty" yaml:"email,omitempty"`
}

// EditLinksConfig represents edit links configuration
type EditLinksConfig struct {
	Enabled  bool                      `json:"enabled" yaml:"enabled"`
	BaseURL  foundation.Option[string] `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	EditText foundation.Option[string] `json:"edit_text,omitempty" yaml:"edit_text,omitempty"`
	PerPage  bool                      `json:"per_page" yaml:"per_page"`
}

// SearchConfig represents search functionality configuration
type SearchConfig struct {
	Enabled   bool                      `json:"enabled" yaml:"enabled"`
	Provider  foundation.Option[string] `json:"provider,omitempty" yaml:"provider,omitempty"`
	IndexPath foundation.Option[string] `json:"index_path,omitempty" yaml:"index_path,omitempty"`
}

// NavigationConfig represents navigation configuration
type NavigationConfig struct {
	ShowTOC        bool `json:"show_toc" yaml:"show_toc"`
	TOCMaxDepth    int  `json:"toc_max_depth" yaml:"toc_max_depth"`
	ShowBreadcrumb bool `json:"show_breadcrumb" yaml:"show_breadcrumb"`
}

// MenuConfig represents Hugo menu configuration
type MenuConfig struct {
	Main   []MenuItem `json:"main,omitempty" yaml:"main,omitempty"`
	Footer []MenuItem `json:"footer,omitempty" yaml:"footer,omitempty"`
}

// MenuItem represents a strongly-typed menu item
type MenuItem struct {
	Name       string                    `json:"name" yaml:"name"`
	URL        string                    `json:"url" yaml:"url"`
	Weight     foundation.Option[int]    `json:"weight,omitempty" yaml:"weight,omitempty"`
	Identifier foundation.Option[string] `json:"identifier,omitempty" yaml:"identifier,omitempty"`
	Parent     foundation.Option[string] `json:"parent,omitempty" yaml:"parent,omitempty"`
	Pre        foundation.Option[string] `json:"pre,omitempty" yaml:"pre,omitempty"`
	Post       foundation.Option[string] `json:"post,omitempty" yaml:"post,omitempty"`
}

// ModuleConfig represents Hugo module configuration
type ModuleConfig struct {
	Imports []ModuleImport `json:"imports" yaml:"imports"`
}

// ModuleImport represents a Hugo module import
type ModuleImport struct {
	Path     string                  `json:"path" yaml:"path"`
	Disabled foundation.Option[bool] `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	Mounts   []ModuleMount           `json:"mounts,omitempty" yaml:"mounts,omitempty"`
}

// ModuleMount represents a Hugo module mount
type ModuleMount struct {
	Source string `json:"source" yaml:"source"`
	Target string `json:"target" yaml:"target"`
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
