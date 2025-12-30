package linkverify

import (
	"time"
)

// BrokenLinkEvent represents a broken link discovered during verification.
// This event is published to NATS for downstream processing (e.g., creating forge issues).
type BrokenLinkEvent struct {
	// Link information
	URL        string `json:"url"`         // The broken link URL
	Status     int    `json:"status"`      // HTTP status code (0 for non-HTTP errors)
	Error      string `json:"error"`       // Error message
	IsInternal bool   `json:"is_internal"` // True if link is internal to the site

	// Source page metadata
	SourcePath         string `json:"source_path"`          // Source file path (absolute)
	SourceRelativePath string `json:"source_relative_path"` // Path relative to docs directory
	Repository         string `json:"repository"`           // Repository name
	Forge              string `json:"forge,omitempty"`      // Forge namespace (if applicable)
	Section            string `json:"section,omitempty"`    // Documentation section
	FileName           string `json:"file_name"`            // File name without extension
	DocsBase           string `json:"docs_base"`            // Configured docs base path

	// Front matter metadata (extracted from markdown)
	Title       string         `json:"title,omitempty"`        // Page title
	Description string         `json:"description,omitempty"`  // Page description
	Date        string         `json:"date,omitempty"`         // Page date
	Type        string         `json:"type,omitempty"`         // Content type
	FrontMatter map[string]any `json:"front_matter,omitempty"` // All front matter fields

	// Generated paths
	HugoPath     string `json:"hugo_path,omitempty"`     // Generated Hugo content path
	RenderedPath string `json:"rendered_path,omitempty"` // Path in rendered site
	RenderedURL  string `json:"rendered_url,omitempty"`  // Full URL of rendered page

	// Verification metadata
	Timestamp     time.Time `json:"timestamp"`                // When the broken link was discovered
	LastChecked   time.Time `json:"last_checked"`             // When this link was last verified
	FailureCount  int       `json:"failure_count"`            // Number of consecutive failures
	FirstFailedAt time.Time `json:"first_failed_at,omitzero"` // When this link first failed

	// Build context
	BuildID   string    `json:"build_id,omitempty"`  // Build identifier
	BuildTime time.Time `json:"build_time,omitzero"` // When the build occurred
}
