package editlink

import (
	"path/filepath"
)

// VSCodeDetector generates edit URLs for local preview mode that trigger VS Code.
// These URLs use the /_edit/ endpoint which opens files in VS Code and redirects back.
type VSCodeDetector struct{}

// NewVSCodeDetector creates a new VS Code detector for local preview.
func NewVSCodeDetector() *VSCodeDetector {
	return &VSCodeDetector{}
}

// Name returns the detector name.
func (d *VSCodeDetector) Name() string {
	return "vscode"
}

// Detect implements Detector for VS Code local preview mode.
func (d *VSCodeDetector) Detect(ctx DetectionContext) DetectionResult {
	// Only activate in local preview mode (repository name is "local")
	if ctx.Repository == nil || ctx.Repository.Name != "local" {
		return DetectionResult{Found: false}
	}

	// The repository URL in preview mode is the local docs directory
	// We need to construct a path relative to that directory

	// ctx.RepoRel is the path relative to the repository root
	// For preview mode, we need to strip the "docs" path component if present
	// since the user might be watching ./docs but we want to open the file relative to that

	relPath := ctx.RepoRel

	// Clean up the path
	relPath = filepath.Clean(relPath)
	relPath = filepath.ToSlash(relPath)

	// Return a special marker that the URL builder will recognize
	return DetectionResult{
		Found:     true,
		ForgeType: "vscode", // Special forge type for local preview
		BaseURL:   "",       // Not used for VS Code URLs
		FullName:  relPath,  // Store the relative path here
	}
}
