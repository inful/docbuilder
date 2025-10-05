package editlink

import (
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// DetectionContext holds all the information needed for forge detection.
type DetectionContext struct {
	File       docs.DocFile
	Config     *config.Config
	Repository *config.Repository
	CloneURL   string
	Branch     string
	RepoRel    string
}

// DetectionResult contains the result of forge detection.
type DetectionResult struct {
	ForgeType config.ForgeType
	BaseURL   string
	FullName  string
	Found     bool
}

// ForgeDetector defines the interface for detecting forge type and details.
type ForgeDetector interface {
	// Detect attempts to determine the forge type and details from the context.
	// Returns a DetectionResult with Found=true if successful, Found=false otherwise.
	Detect(ctx DetectionContext) DetectionResult

	// Name returns a human-readable name for this detector (for debugging/logging).
	Name() string
}

// DetectorChain implements a chain of responsibility pattern for forge detection.
type DetectorChain struct {
	detectors []ForgeDetector
}

// NewDetectorChain creates a new detector chain.
func NewDetectorChain() *DetectorChain {
	return &DetectorChain{
		detectors: make([]ForgeDetector, 0),
	}
}

// Add appends a detector to the chain.
func (dc *DetectorChain) Add(detector ForgeDetector) *DetectorChain {
	dc.detectors = append(dc.detectors, detector)
	return dc
}

// Detect runs through the chain of detectors until one succeeds.
func (dc *DetectorChain) Detect(ctx DetectionContext) DetectionResult {
	for _, detector := range dc.detectors {
		if result := detector.Detect(ctx); result.Found {
			return result
		}
	}
	return DetectionResult{Found: false}
}

// BuildEditURL constructs the final edit URL from detection results.
type EditURLBuilder interface {
	BuildURL(forgeType config.ForgeType, baseURL, fullName, branch, repoRel string) string
}

// Context preparation utilities

// PrepareDetectionContext creates a DetectionContext from a DocFile and config.
func PrepareDetectionContext(file docs.DocFile, cfg *config.Config) (DetectionContext, bool) {
	// Find repository configuration
	var repoCfg *config.Repository
	for i := range cfg.Repositories {
		if cfg.Repositories[i].Name == file.Repository {
			repoCfg = &cfg.Repositories[i]
			break
		}
	}
	if repoCfg == nil {
		return DetectionContext{}, false
	}

	// Determine branch
	branch := repoCfg.Branch
	if branch == "" {
		branch = "main"
	}

	// Calculate repository-relative path
	repoRel := prepareRepoRelativePath(file)

	// Clean clone URL
	cloneURL := prepareCloneURL(repoCfg.URL)

	return DetectionContext{
		File:       file,
		Config:     cfg,
		Repository: repoCfg,
		CloneURL:   cloneURL,
		Branch:     branch,
		RepoRel:    repoRel,
	}, true
}

// prepareRepoRelativePath calculates the repository-relative path for the file.
func prepareRepoRelativePath(file docs.DocFile) string {
	repoRel := file.RelativePath
	if base := strings.TrimSpace(file.DocsBase); base != "" && base != "." {
		repoRel = filepath.ToSlash(filepath.Join(base, repoRel))
	} else {
		repoRel = filepath.ToSlash(repoRel)
	}
	return repoRel
}

// prepareCloneURL normalizes a clone URL by removing .git suffix.
func prepareCloneURL(url string) string {
	if url == "" {
		return ""
	}
	return strings.TrimSuffix(url, ".git")
}
