package typed

import (
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
			foundation.ValidationError(fmt.Sprintf("invalid theme type: %s", s)).
				WithContext(foundation.Fields{
					"input":        s,
					"valid_values": []string{"hextra", "docsy", "book", "custom"},
				}).
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

	hugoMarkupNormalizer = foundation.NewNormalizer(map[string]HugoMarkupType{
		"goldmark":    HugoMarkupGoldmark,
		"blackfriday": HugoMarkupBlackfriday,
	}, HugoMarkupGoldmark)
)

func (hm HugoMarkupType) String() string {
	return hm.value
}

// TypedHugoConfig represents a strongly-typed Hugo configuration
type TypedHugoConfig struct {
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
	Params TypedHugoParams `yaml:"params,omitempty" json:"params,omitempty"`

	// Menu configuration
	Menu TypedMenuConfig `yaml:"menu,omitempty" json:"menu,omitempty"`

	// Module configuration (for themes that support it)
	Module foundation.Option[TypedModuleConfig] `yaml:"module,omitempty" json:"module,omitempty"`

	// Custom settings for advanced configurations
	CustomConfig map[string]any `yaml:"custom,omitempty" json:"custom,omitempty"`
}

// TypedHugoParams represents strongly-typed Hugo theme parameters
type TypedHugoParams struct {
	// Common theme parameters
	Author      foundation.Option[string] `yaml:"author,omitempty" json:"author,omitempty"`
	Description foundation.Option[string] `yaml:"description,omitempty" json:"description,omitempty"`
	Keywords    []string                  `yaml:"keywords,omitempty" json:"keywords,omitempty"`

	// Social and metadata
	Social foundation.Option[TypedSocialConfig] `yaml:"social,omitempty" json:"social,omitempty"`

	// Edit links configuration
	EditLinks TypedEditLinksConfig `yaml:"edit_links" json:"edit_links"`

	// Search configuration
	Search foundation.Option[TypedSearchConfig] `yaml:"search,omitempty" json:"search,omitempty"`

	// Navigation configuration
	Navigation TypedNavigationConfig `yaml:"navigation" json:"navigation"`

	// Theme-specific parameters (organized by theme)
	Hextra foundation.Option[TypedHextraParams] `yaml:"hextra,omitempty" json:"hextra,omitempty"`
	Docsy  foundation.Option[TypedDocsyParams]  `yaml:"docsy,omitempty" json:"docsy,omitempty"`

	// Custom parameters for extensibility
	Custom map[string]any `yaml:"custom,omitempty" json:"custom,omitempty"`
}

// TypedSocialConfig represents social media configuration
type TypedSocialConfig struct {
	GitHub   foundation.Option[string] `yaml:"github,omitempty" json:"github,omitempty"`
	Twitter  foundation.Option[string] `yaml:"twitter,omitempty" json:"twitter,omitempty"`
	LinkedIn foundation.Option[string] `yaml:"linkedin,omitempty" json:"linkedin,omitempty"`
	Email    foundation.Option[string] `yaml:"email,omitempty" json:"email,omitempty"`
}

// TypedEditLinksConfig represents edit links configuration
type TypedEditLinksConfig struct {
	Enabled  bool                      `yaml:"enabled" json:"enabled"`
	BaseURL  foundation.Option[string] `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	EditText foundation.Option[string] `yaml:"edit_text,omitempty" json:"edit_text,omitempty"`
	PerPage  bool                      `yaml:"per_page" json:"per_page"`
}

// TypedSearchConfig represents search functionality configuration
type TypedSearchConfig struct {
	Enabled   bool                      `yaml:"enabled" json:"enabled"`
	Provider  foundation.Option[string] `yaml:"provider,omitempty" json:"provider,omitempty"`
	IndexPath foundation.Option[string] `yaml:"index_path,omitempty" json:"index_path,omitempty"`
}

// TypedNavigationConfig represents navigation configuration
type TypedNavigationConfig struct {
	ShowTOC        bool `yaml:"show_toc" json:"show_toc"`
	TOCMaxDepth    int  `yaml:"toc_max_depth" json:"toc_max_depth"`
	ShowBreadcrumb bool `yaml:"show_breadcrumb" json:"show_breadcrumb"`
}

// TypedMenuConfig represents Hugo menu configuration
type TypedMenuConfig struct {
	Main   []TypedMenuItem `yaml:"main,omitempty" json:"main,omitempty"`
	Footer []TypedMenuItem `yaml:"footer,omitempty" json:"footer,omitempty"`
}

// TypedMenuItem represents a strongly-typed menu item
type TypedMenuItem struct {
	Name       string                    `yaml:"name" json:"name"`
	URL        string                    `yaml:"url" json:"url"`
	Weight     foundation.Option[int]    `yaml:"weight,omitempty" json:"weight,omitempty"`
	Identifier foundation.Option[string] `yaml:"identifier,omitempty" json:"identifier,omitempty"`
	Parent     foundation.Option[string] `yaml:"parent,omitempty" json:"parent,omitempty"`
	Pre        foundation.Option[string] `yaml:"pre,omitempty" json:"pre,omitempty"`
	Post       foundation.Option[string] `yaml:"post,omitempty" json:"post,omitempty"`
}

// TypedModuleConfig represents Hugo module configuration
type TypedModuleConfig struct {
	Imports []TypedModuleImport `yaml:"imports" json:"imports"`
}

// TypedModuleImport represents a Hugo module import
type TypedModuleImport struct {
	Path     string                  `yaml:"path" json:"path"`
	Disabled foundation.Option[bool] `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Mounts   []TypedModuleMount      `yaml:"mounts,omitempty" json:"mounts,omitempty"`
}

// TypedModuleMount represents a Hugo module mount
type TypedModuleMount struct {
	Source string `yaml:"source" json:"source"`
	Target string `yaml:"target" json:"target"`
}

// Theme-specific parameter types

// TypedHextraParams represents Hextra theme specific parameters
type TypedHextraParams struct {
	DisplayMode foundation.Option[string]            `yaml:"displayMode,omitempty" json:"displayMode,omitempty"`
	Width       foundation.Option[string]            `yaml:"width,omitempty" json:"width,omitempty"`
	Navbar      foundation.Option[TypedHextraNavbar] `yaml:"navbar,omitempty" json:"navbar,omitempty"`
	Footer      foundation.Option[TypedHextraFooter] `yaml:"footer,omitempty" json:"footer,omitempty"`
}

// TypedHextraNavbar represents Hextra navbar configuration
type TypedHextraNavbar struct {
	DisplayTitle bool                      `yaml:"displayTitle" json:"displayTitle"`
	DisplayLogo  bool                      `yaml:"displayLogo" json:"displayLogo"`
	LogoPath     foundation.Option[string] `yaml:"logoPath,omitempty" json:"logoPath,omitempty"`
}

// TypedHextraFooter represents Hextra footer configuration
type TypedHextraFooter struct {
	Enable      bool                      `yaml:"enable" json:"enable"`
	DisplayText foundation.Option[string] `yaml:"displayText,omitempty" json:"displayText,omitempty"`
}

// TypedDocsyParams represents Docsy theme specific parameters
type TypedDocsyParams struct {
	EditPage TypedDocsyEditPage              `yaml:"edit_page" json:"edit_page"`
	Search   TypedDocsySearch                `yaml:"search" json:"search"`
	Taxonomy TypedDocsyTaxonomy              `yaml:"taxonomy" json:"taxonomy"`
	UI       foundation.Option[TypedDocsyUI] `yaml:"ui,omitempty" json:"ui,omitempty"`
}

// TypedDocsyEditPage represents Docsy edit page configuration
type TypedDocsyEditPage struct {
	ViewURL foundation.Option[string] `yaml:"view_url,omitempty" json:"view_url,omitempty"`
	EditURL foundation.Option[string] `yaml:"edit_url,omitempty" json:"edit_url,omitempty"`
}

// TypedDocsySearch represents Docsy search configuration
type TypedDocsySearch struct {
	Algolia foundation.Option[TypedDocsyAlgolia] `yaml:"algolia,omitempty" json:"algolia,omitempty"`
}

// TypedDocsyAlgolia represents Docsy Algolia search configuration
type TypedDocsyAlgolia struct {
	AppID     foundation.Option[string] `yaml:"appId,omitempty" json:"appId,omitempty"`
	APIKey    foundation.Option[string] `yaml:"apiKey,omitempty" json:"apiKey,omitempty"`
	IndexName foundation.Option[string] `yaml:"indexName,omitempty" json:"indexName,omitempty"`
}

// TypedDocsyTaxonomy represents Docsy taxonomy configuration
type TypedDocsyTaxonomy struct {
	TaxonomyCloud      []string `yaml:"taxonomyCloud,omitempty" json:"taxonomyCloud,omitempty"`
	TaxonomyCloudTitle []string `yaml:"taxonomyCloudTitle,omitempty" json:"taxonomyCloudTitle,omitempty"`
}

// TypedDocsyUI represents Docsy UI configuration
type TypedDocsyUI struct {
	ShowVisitedLinks   bool `yaml:"showVisitedLinks" json:"showVisitedLinks"`
	SidebarMenuCompact bool `yaml:"sidebar_menu_compact" json:"sidebar_menu_compact"`
	BreadcrumbDisable  bool `yaml:"breadcrumb_disable" json:"breadcrumb_disable"`
}

// Validation methods for TypedHugoConfig

// Validate performs comprehensive validation of the Hugo configuration
func (hc *TypedHugoConfig) Validate() foundation.ValidationResult {
	chain := foundation.NewValidatorChain(
		// Validate theme type
		func(config TypedHugoConfig) foundation.ValidationResult {
			return hugoThemeValidator(config.Theme)
		},

		// Validate title is not empty
		func(config TypedHugoConfig) foundation.ValidationResult {
			if strings.TrimSpace(config.Title) == "" {
				return foundation.Invalid(
					foundation.NewValidationError("title", "not_empty", "title cannot be empty"),
				)
			}
			return foundation.Valid()
		},

		// Validate baseURL format if provided
		func(config TypedHugoConfig) foundation.ValidationResult {
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
		func(config TypedHugoConfig) foundation.ValidationResult {
			if config.ContentDir != "" && !isValidPath(config.ContentDir) {
				return foundation.Invalid(
					foundation.NewValidationError("contentDir", "valid_path",
						"contentDir must be a valid relative path"),
				)
			}
			return foundation.Valid()
		},

		// Validate publish directory path
		func(config TypedHugoConfig) foundation.ValidationResult {
			if config.PublishDir != "" && !isValidPath(config.PublishDir) {
				return foundation.Invalid(
					foundation.NewValidationError("publishDir", "valid_path",
						"publishDir must be a valid relative path"),
				)
			}
			return foundation.Valid()
		},

		// Validate theme-specific configuration
		func(config TypedHugoConfig) foundation.ValidationResult {
			return config.validateThemeSpecificConfig()
		},
	)

	return chain.Validate(*hc)
}

// validateThemeSpecificConfig validates theme-specific parameters
func (hc *TypedHugoConfig) validateThemeSpecificConfig() foundation.ValidationResult {
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
func (hp *TypedHextraParams) Validate() foundation.ValidationResult {
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
func (dp *TypedDocsyParams) Validate() foundation.ValidationResult {
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

// ConversionMethods for backward compatibility

// ToLegacyMap converts TypedHugoConfig to map[string]any for legacy compatibility
func (hc *TypedHugoConfig) ToLegacyMap() map[string]any {
	result := make(map[string]any)

	result["title"] = hc.Title
	result["theme"] = hc.Theme.String()

	if hc.BaseURL.IsSome() {
		result["baseURL"] = hc.BaseURL.Unwrap()
	}

	if hc.ContentDir != "" {
		result["contentDir"] = hc.ContentDir
	}

	if hc.PublishDir != "" {
		result["publishDir"] = hc.PublishDir
	}

	if len(hc.StaticDir) > 0 {
		result["staticDir"] = hc.StaticDir
	}

	if hc.LanguageCode.IsSome() {
		result["languageCode"] = hc.LanguageCode.Unwrap()
	}

	result["buildDrafts"] = hc.BuildDrafts
	result["buildFuture"] = hc.BuildFuture
	result["buildExpired"] = hc.BuildExpired

	// Add params
	paramsMap := make(map[string]any)

	if hc.Params.Author.IsSome() {
		paramsMap["author"] = hc.Params.Author.Unwrap()
	}

	if hc.Params.Description.IsSome() {
		paramsMap["description"] = hc.Params.Description.Unwrap()
	}

	if len(hc.Params.Keywords) > 0 {
		paramsMap["keywords"] = hc.Params.Keywords
	}

	// Add theme-specific params
	switch hc.Theme {
	case HugoThemeHextra:
		if hc.Params.Hextra.IsSome() {
			hextraParams := hc.Params.Hextra.Unwrap()
			if hextraParams.DisplayMode.IsSome() {
				paramsMap["displayMode"] = hextraParams.DisplayMode.Unwrap()
			}
		}
	case HugoThemeDocsy:
		if hc.Params.Docsy.IsSome() {
			docsyParams := hc.Params.Docsy.Unwrap()
			docsyMap := make(map[string]any)

			editPageMap := make(map[string]any)
			if docsyParams.EditPage.ViewURL.IsSome() {
				editPageMap["view_url"] = docsyParams.EditPage.ViewURL.Unwrap()
			}
			if docsyParams.EditPage.EditURL.IsSome() {
				editPageMap["edit_url"] = docsyParams.EditPage.EditURL.Unwrap()
			}
			if len(editPageMap) > 0 {
				docsyMap["edit_page"] = editPageMap
			}

			paramsMap["docsy"] = docsyMap
		}
	}

	// Add custom params
	for k, v := range hc.Params.Custom {
		paramsMap[k] = v
	}

	if len(paramsMap) > 0 {
		result["params"] = paramsMap
	}

	// Add custom config
	for k, v := range hc.CustomConfig {
		result[k] = v
	}

	return result
}

// FromLegacyMap creates a TypedHugoConfig from a legacy map[string]any
func FromLegacyMap(data map[string]any) foundation.Result[TypedHugoConfig, error] {
	config := TypedHugoConfig{
		ContentDir: "content",
		PublishDir: "public",
		MarkupType: HugoMarkupGoldmark,
		Params: TypedHugoParams{
			EditLinks: TypedEditLinksConfig{
				Enabled: true,
				PerPage: true,
			},
			Navigation: TypedNavigationConfig{
				ShowTOC:        true,
				TOCMaxDepth:    3,
				ShowBreadcrumb: true,
			},
		},
	}

	// Extract title
	if title, ok := data["title"].(string); ok {
		config.Title = title
	} else {
		return foundation.Err[TypedHugoConfig, error](
			fmt.Errorf("title is required and must be a string"),
		)
	}

	// Extract theme
	if themeStr, ok := data["theme"].(string); ok {
		themeResult := ParseHugoThemeType(themeStr)
		if themeResult.IsErr() {
			return foundation.Err[TypedHugoConfig, error](themeResult.UnwrapErr())
		}
		config.Theme = themeResult.Unwrap()
	} else {
		config.Theme = HugoThemeHextra // default
	}

	// Extract baseURL
	if baseURL, ok := data["baseURL"].(string); ok && baseURL != "" {
		config.BaseURL = foundation.Some(baseURL)
	}

	// Extract other string fields
	if contentDir, ok := data["contentDir"].(string); ok {
		config.ContentDir = contentDir
	}

	if publishDir, ok := data["publishDir"].(string); ok {
		config.PublishDir = publishDir
	}

	// Extract boolean fields
	if buildDrafts, ok := data["buildDrafts"].(bool); ok {
		config.BuildDrafts = buildDrafts
	}

	if buildFuture, ok := data["buildFuture"].(bool); ok {
		config.BuildFuture = buildFuture
	}

	if buildExpired, ok := data["buildExpired"].(bool); ok {
		config.BuildExpired = buildExpired
	}

	// Extract params
	if paramsData, ok := data["params"].(map[string]any); ok {
		if author, ok := paramsData["author"].(string); ok {
			config.Params.Author = foundation.Some(author)
		}

		if description, ok := paramsData["description"].(string); ok {
			config.Params.Description = foundation.Some(description)
		}

		if keywords, ok := paramsData["keywords"].([]string); ok {
			config.Params.Keywords = keywords
		} else if keywordsInterface, ok := paramsData["keywords"].([]interface{}); ok {
			// Convert []interface{} to []string
			keywords := make([]string, 0, len(keywordsInterface))
			for _, k := range keywordsInterface {
				if kStr, ok := k.(string); ok {
					keywords = append(keywords, kStr)
				}
			}
			config.Params.Keywords = keywords
		}

		// Store remaining params as custom
		config.Params.Custom = make(map[string]any)
		for k, v := range paramsData {
			switch k {
			case "author", "description", "keywords":
				// Already handled above
			default:
				config.Params.Custom[k] = v
			}
		}
	}

	// Store any remaining top-level fields as custom config
	config.CustomConfig = make(map[string]any)
	for k, v := range data {
		switch k {
		case "title", "theme", "baseURL", "contentDir", "publishDir", "buildDrafts", "buildFuture", "buildExpired", "params":
			// Already handled above
		default:
			config.CustomConfig[k] = v
		}
	}

	// Validate the constructed configuration
	if validationResult := config.Validate(); !validationResult.Valid {
		return foundation.Err[TypedHugoConfig, error](
			fmt.Errorf("configuration validation failed: %v", validationResult.Errors),
		)
	}

	return foundation.Ok[TypedHugoConfig, error](config)
}
