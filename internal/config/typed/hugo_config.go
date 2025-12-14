package typed

import (
"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// HugoThemeType represents strongly-typed Hugo theme options
type HugoThemeType struct {
	value string
}

// Predefined Hugo theme types
var (
	HugoThemeHextra = HugoThemeType{"hextra"}
	HugoThemeDocsy  = HugoThemeType{"docsy"}
	HugoThemeBook   = HugoThemeType{"book"}
	HugoThemeCustom = HugoThemeType{"custom"}

	// Normalizer for theme types
	hugoThemeNormalizer = foundation.NewNormalizer(map[string]HugoThemeType{
		"hextra": HugoThemeHextra,
		"docsy":  HugoThemeDocsy,
		"book":   HugoThemeBook,
		"custom": HugoThemeCustom,
	}, HugoThemeHextra) // default to Hextra

	// Validator for theme types
	hugoThemeValidator = foundation.OneOf("theme_type", []HugoThemeType{
		HugoThemeHextra, HugoThemeDocsy, HugoThemeBook, HugoThemeCustom,
	})
)

// String returns the string representation of the theme type
func (ht HugoThemeType) String() string {
	return ht.value
}

// Valid checks if the theme type is valid
func (ht HugoThemeType) Valid() bool {
	return hugoThemeValidator(ht).Valid
}

// SupportsModules indicates if this theme supports Hugo modules
func (ht HugoThemeType) SupportsModules() bool {
	switch ht {
	case HugoThemeHextra, HugoThemeDocsy:
		return true
	default:
		return false
	}
}

// GetModulePath returns the Hugo module path for this theme
func (ht HugoThemeType) GetModulePath() foundation.Option[string] {
	switch ht {
	case HugoThemeHextra:
		return foundation.Some("github.com/imfing/hextra")
	case HugoThemeDocsy:
		return foundation.Some("github.com/google/docsy")
	default:
		return foundation.None[string]()
	}
}

// ParseHugoThemeType parses a string into a HugoThemeType
func ParseHugoThemeType(s string) foundation.Result[HugoThemeType, error] {
	theme, err := hugoThemeNormalizer.NormalizeWithError(s)
	if err != nil {
		return foundation.Err[HugoThemeType, error](
			errors.ValidationError(fmt.Sprintf("invalid theme type: %s", s)).
WithContext("input", s).
WithContext("valid_values", []string{"hextra", "docsy", "book", "custom"}).
				Build(),
		)
	}
	return foundation.Ok[HugoThemeType, error](theme)
}

// NormalizeHugoThemeType normalizes a string to a HugoThemeType
func NormalizeHugoThemeType(s string) HugoThemeType {
	return hugoThemeNormalizer.Normalize(s)
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
	Theme   HugoThemeType             `yaml:"theme" json:"theme"`

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

	// Theme-specific parameters (organized by theme)
	Hextra foundation.Option[HextraParams] `yaml:"hextra,omitempty" json:"hextra,omitempty"`
	Docsy  foundation.Option[DocsyParams]  `yaml:"docsy,omitempty" json:"docsy,omitempty"`

	// Custom parameters for extensibility
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

// Theme-specific parameter types

// HextraParams represents Hextra theme specific parameters
type HextraParams struct {
	DisplayMode foundation.Option[string]       `yaml:"displayMode,omitempty" json:"displayMode,omitempty"`
	Width       foundation.Option[string]       `yaml:"width,omitempty" json:"width,omitempty"`
	Navbar      foundation.Option[HextraNavbar] `yaml:"navbar,omitempty" json:"navbar,omitempty"`
	Footer      foundation.Option[HextraFooter] `yaml:"footer,omitempty" json:"footer,omitempty"`
}

// HextraNavbar represents Hextra navbar configuration
type HextraNavbar struct {
	DisplayTitle bool                      `yaml:"displayTitle" json:"displayTitle"`
	DisplayLogo  bool                      `yaml:"displayLogo" json:"displayLogo"`
	LogoPath     foundation.Option[string] `yaml:"logoPath,omitempty" json:"logoPath,omitempty"`
}

// HextraFooter represents Hextra footer configuration
type HextraFooter struct {
	Enable      bool                      `yaml:"enable" json:"enable"`
	DisplayText foundation.Option[string] `yaml:"displayText,omitempty" json:"displayText,omitempty"`
}

// DocsyParams represents Docsy theme specific parameters
type DocsyParams struct {
	EditPage DocsyEditPage              `yaml:"edit_page" json:"edit_page"`
	Search   DocsySearch                `yaml:"search" json:"search"`
	Taxonomy DocsyTaxonomy              `yaml:"taxonomy" json:"taxonomy"`
	UI       foundation.Option[DocsyUI] `yaml:"ui,omitempty" json:"ui,omitempty"`
}

// DocsyEditPage represents Docsy edit page configuration
type DocsyEditPage struct {
	ViewURL foundation.Option[string] `yaml:"view_url,omitempty" json:"view_url,omitempty"`
	EditURL foundation.Option[string] `yaml:"edit_url,omitempty" json:"edit_url,omitempty"`
}

// DocsySearch represents Docsy search configuration
type DocsySearch struct {
	Algolia foundation.Option[DocsyAlgolia] `yaml:"algolia,omitempty" json:"algolia,omitempty"`
}

// DocsyAlgolia represents Docsy Algolia search configuration
type DocsyAlgolia struct {
	AppID     foundation.Option[string] `yaml:"appId,omitempty" json:"appId,omitempty"`
	APIKey    foundation.Option[string] `yaml:"apiKey,omitempty" json:"apiKey,omitempty"`
	IndexName foundation.Option[string] `yaml:"indexName,omitempty" json:"indexName,omitempty"`
}

// DocsyTaxonomy represents Docsy taxonomy configuration
type DocsyTaxonomy struct {
	TaxonomyCloud      []string `yaml:"taxonomyCloud,omitempty" json:"taxonomyCloud,omitempty"`
	TaxonomyCloudTitle []string `yaml:"taxonomyCloudTitle,omitempty" json:"taxonomyCloudTitle,omitempty"`
}

// DocsyUI represents Docsy UI configuration
type DocsyUI struct {
	ShowVisitedLinks   bool `yaml:"showVisitedLinks" json:"showVisitedLinks"`
	SidebarMenuCompact bool `yaml:"sidebar_menu_compact" json:"sidebar_menu_compact"`
	BreadcrumbDisable  bool `yaml:"breadcrumb_disable" json:"breadcrumb_disable"`
}

// Validation methods for TypedHugoConfig

// Validate performs comprehensive validation of the Hugo configuration
func (hc *HugoConfig) Validate() foundation.ValidationResult {
	chain := foundation.NewValidatorChain(
		// Validate theme type
		func(config HugoConfig) foundation.ValidationResult {
			return hugoThemeValidator(config.Theme)
		},

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

		// Validate theme-specific configuration
		func(config HugoConfig) foundation.ValidationResult {
			return config.validateThemeSpecificConfig()
		},
	)

	return chain.Validate(*hc)
}

// validateThemeSpecificConfig validates theme-specific parameters
func (hc *HugoConfig) validateThemeSpecificConfig() foundation.ValidationResult {
	switch hc.Theme {
	case HugoThemeHextra:
		if hc.Params.Hextra.IsSome() {
			hextraParams := hc.Params.Hextra.Unwrap()
			return hextraParams.Validate()
		}
	case HugoThemeDocsy:
		if hc.Params.Docsy.IsSome() {
			docsyParams := hc.Params.Docsy.Unwrap()
			return docsyParams.Validate()
		}
	}
	return foundation.Valid()
}

// Validate methods for theme-specific parameters

// Validate validates Hextra theme parameters
func (hp *HextraParams) Validate() foundation.ValidationResult {
	// Validate display mode if specified
	if hp.DisplayMode.IsSome() {
		mode := hp.DisplayMode.Unwrap()
		validModes := []string{"light", "dark", "auto"}
		isValid := false
		for _, valid := range validModes {
			if mode == valid {
				isValid = true
				break
			}
		}
		if !isValid {
			return foundation.Invalid(
				foundation.NewValidationError("displayMode", "valid_option",
					fmt.Sprintf("displayMode must be one of: %v", validModes)),
			)
		}
	}
	return foundation.Valid()
}

// Validate validates Docsy theme parameters
func (dp *DocsyParams) Validate() foundation.ValidationResult {
	// For now, just validate that URLs are properly formatted if provided
	if dp.EditPage.ViewURL.IsSome() {
		if _, err := url.Parse(dp.EditPage.ViewURL.Unwrap()); err != nil {
			return foundation.Invalid(
				foundation.NewValidationError("edit_page.view_url", "valid_url",
					fmt.Sprintf("view_url must be a valid URL: %v", err)),
			)
		}
	}

	if dp.EditPage.EditURL.IsSome() {
		if _, err := url.Parse(dp.EditPage.EditURL.Unwrap()); err != nil {
			return foundation.Invalid(
				foundation.NewValidationError("edit_page.edit_url", "valid_url",
					fmt.Sprintf("edit_url must be a valid URL: %v", err)),
			)
		}
	}

	return foundation.Valid()
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
