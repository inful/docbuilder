// Package editlink resolves edit URLs for documentation files.
//
// The package exposes a single Resolver type. Resolution runs a fixed chain
// of detectors in priority order:
//
//  1. VSCode (local preview mode)
//  2. Configured (repository tags)
//  3. ForgeConfig (configured forge base URLs)
//  4. Heuristic (hostname pattern matching)
//
// The first detector that returns Found=true wins; the resolver then builds
// the URL using forge.GenerateEditURL (with special handling for VS Code and
// Bitbucket).
package editlink

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// Resolver provides edit link resolution.
type Resolver struct{}

// NewResolver creates a new edit link resolver with the standard detector chain.
func NewResolver() *Resolver {
	return &Resolver{}
}

// detectionContext holds the resolved inputs for a single Resolve call.
type detectionContext struct {
	config     *config.Config
	repository *config.Repository
	cloneURL   string
	branch     string
	repoRel    string
}

// detectionResult is the output of a single detector step.
type detectionResult struct {
	ForgeType config.ForgeType
	BaseURL   string
	FullName  string
	Found     bool
}

// Resolve determines the edit URL for a DocFile.
// Returns empty string if edit links should not be generated.
func (r *Resolver) Resolve(file docs.DocFile, cfg *config.Config) string {
	if !shouldGenerateEditLink(cfg) {
		return ""
	}
	if isSiteLevelSuppressed(cfg) {
		return ""
	}

	ctx, ok := prepareDetectionContext(file, cfg)
	if !ok {
		return ""
	}

	if result := detectVSCode(ctx); result.Found {
		return buildURL(result, ctx)
	}
	if result := detectConfigured(ctx); result.Found {
		return buildURL(result, ctx)
	}
	if result := detectForgeConfig(ctx); result.Found {
		return buildURL(result, ctx)
	}
	if result := detectHeuristic(ctx); result.Found {
		return buildURL(result, ctx)
	}
	return ""
}

// shouldGenerateEditLink checks if edit links should be generated at all.
func shouldGenerateEditLink(cfg *config.Config) bool {
	// Edit links now work for all themes.
	return cfg != nil
}

// isSiteLevelSuppressed checks if edit links are suppressed at the site level.
func isSiteLevelSuppressed(cfg *config.Config) bool {
	if cfg.Hugo.Params == nil {
		return false
	}

	editURL, exists := cfg.Hugo.Params["editURL"]
	if !exists {
		return false
	}

	editURLMap, ok := editURL.(map[string]any)
	if !ok {
		return false
	}

	base, exists := editURLMap["base"].(string)
	return exists && base != ""
}

// prepareDetectionContext creates a detectionContext from a DocFile and config.
func prepareDetectionContext(file docs.DocFile, cfg *config.Config) (detectionContext, bool) {
	var repoCfg *config.Repository
	for i := range cfg.Repositories {
		if cfg.Repositories[i].Name == file.Repository {
			repoCfg = &cfg.Repositories[i]
			break
		}
	}
	if repoCfg == nil {
		return detectionContext{}, false
	}

	branch := repoCfg.Branch
	if branch == "" {
		branch = "main"
	}

	return detectionContext{
		config:     cfg,
		repository: repoCfg,
		cloneURL:   prepareCloneURL(repoCfg.URL),
		branch:     branch,
		repoRel:    prepareRepoRelativePath(file),
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
func prepareCloneURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	return strings.TrimSuffix(rawURL, ".git")
}

// detectVSCode handles local preview mode (repository name "local").
func detectVSCode(ctx detectionContext) detectionResult {
	if ctx.repository == nil || ctx.repository.Name != "local" {
		return detectionResult{Found: false}
	}

	relPath := filepath.ToSlash(filepath.Clean(ctx.repoRel))
	return detectionResult{
		Found:     true,
		ForgeType: "vscode",
		FullName:  relPath,
	}
}

// detectConfigured extracts forge info from repository tags.
func detectConfigured(ctx detectionContext) detectionResult {
	if ctx.repository == nil || ctx.repository.Tags == nil {
		return detectionResult{Found: false}
	}

	tags := ctx.repository.Tags

	var forgeType config.ForgeType
	if t, ok := tags["forge_type"]; ok {
		forgeType = config.NormalizeForgeType(t)
	}

	fullName := tags["full_name"]
	if fullName == "" {
		return detectionResult{Found: false}
	}

	baseURL := ""
	if forgeType != "" && ctx.config != nil {
		for _, forge := range ctx.config.Forges {
			if forge != nil && forge.Type == forgeType {
				baseURL = forge.BaseURL
				break
			}
		}
	}

	if forgeType == "" {
		return detectionResult{Found: false}
	}

	return detectionResult{
		ForgeType: forgeType,
		BaseURL:   baseURL,
		FullName:  fullName,
		Found:     true,
	}
}

// detectForgeConfig matches the repository URL against configured forge base URLs.
func detectForgeConfig(ctx detectionContext) detectionResult {
	forgeType, baseURL := resolveForgeForRepository(ctx.config, ctx.cloneURL)
	if forgeType == "" {
		return detectionResult{Found: false}
	}

	fullName := extractFullNameFromURL(ctx.cloneURL)
	if fullName == "" {
		return detectionResult{Found: false}
	}

	return detectionResult{
		ForgeType: forgeType,
		BaseURL:   baseURL,
		FullName:  fullName,
		Found:     true,
	}
}

// resolveForgeForRepository matches a clone URL against configured forge base URLs.
func resolveForgeForRepository(cfg *config.Config, repoURL string) (config.ForgeType, string) {
	if cfg == nil || len(cfg.Forges) == 0 || repoURL == "" {
		return "", ""
	}

	normalized := normalizeSSHURL(repoURL)

	for _, fc := range cfg.Forges {
		if fc == nil || fc.BaseURL == "" {
			continue
		}

		base := strings.TrimSuffix(fc.BaseURL, "/")
		if strings.HasPrefix(normalized, base+"/") || strings.HasPrefix(normalized, base) {
			return fc.Type, base
		}
		if hostsMatch(base, normalized) {
			return fc.Type, base
		}
	}

	return "", ""
}

// detectHeuristic uses hostname patterns to determine forge type.
func detectHeuristic(ctx detectionContext) detectionResult {
	cloneURL := ctx.cloneURL
	if cloneURL == "" {
		return detectionResult{Found: false}
	}
	if isLocalPath(cloneURL) {
		return detectionResult{Found: false}
	}

	forgeType := detectForgeTypeFromHost(cloneURL)
	if forgeType == "" {
		return detectionResult{Found: false}
	}

	fullName := extractFullNameFromURL(cloneURL)
	if fullName == "" {
		return detectionResult{Found: false}
	}

	return detectionResult{
		ForgeType: forgeType,
		BaseURL:   determineBaseURL(cloneURL, forgeType),
		FullName:  fullName,
		Found:     true,
	}
}

// detectForgeTypeFromHost determines forge type based on hostname patterns.
func detectForgeTypeFromHost(cloneURL string) config.ForgeType {
	switch {
	case strings.Contains(cloneURL, "github."):
		return config.ForgeGitHub
	case strings.Contains(cloneURL, "gitlab."):
		return config.ForgeGitLab
	case strings.Contains(cloneURL, "bitbucket.org"):
		// Special case: Bitbucket uses custom URL format.
		return config.ForgeForgejo // Placeholder since Bitbucket isn't defined.
	case strings.Contains(cloneURL, "forgejo") || strings.Contains(cloneURL, "gitea"):
		return config.ForgeForgejo
	default:
		// Assume Forgejo/Gitea for unknown self-hosted instances.
		return config.ForgeForgejo
	}
}

// determineBaseURL calculates the base URL for a given clone URL and forge type.
func determineBaseURL(cloneURL string, forgeType config.ForgeType) string {
	normalized := normalizeSSHURL(cloneURL)

	if strings.Contains(cloneURL, "bitbucket.org") {
		return "https://bitbucket.org"
	}

	if u, err := url.Parse(normalized); err == nil && u.Scheme != "" && u.Host != "" {
		return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	switch forgeType {
	case config.ForgeGitHub:
		return "https://github.com"
	case config.ForgeGitLab:
		return "https://gitlab.com"
	case config.ForgeForgejo, config.ForgeLocal:
		// For self-hosted/local forges, use the clone URL as base.
		return cloneURL
	default:
		return ""
	}
}

// isLocalPath checks if a URL is a local file path (not a remote git URL).
func isLocalPath(urlStr string) bool {
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		return false
	}
	if strings.HasPrefix(urlStr, "git@") || strings.HasPrefix(urlStr, "ssh://") {
		return false
	}
	if strings.HasPrefix(urlStr, "git://") {
		return false
	}
	return true
}

// normalizeSSHURL converts SSH URLs to HTTPS format for easier parsing.
func normalizeSSHURL(repoURL string) string {
	if !strings.HasPrefix(repoURL, "git@") {
		return repoURL
	}

	parts := strings.SplitN(strings.TrimPrefix(repoURL, "git@"), ":", 2)
	if len(parts) == 2 {
		return "https://" + parts[0] + "/" + parts[1]
	}

	return repoURL
}

// hostsMatch checks if two URLs have the same host.
func hostsMatch(url1, url2 string) bool {
	u1, err1 := url.Parse(url1)
	u2, err2 := url.Parse(url2)
	if err1 != nil || err2 != nil {
		return false
	}
	return u1.Host != "" && u1.Host == u2.Host
}

// extractFullNameFromURL extracts the repository full name (owner/repo) from a URL.
func extractFullNameFromURL(cloneURL string) string {
	normalized := normalizeSSHURL(cloneURL)

	u, err := url.Parse(normalized)
	if err != nil {
		return ""
	}

	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	return path
}

// buildURL constructs the final edit URL from a detection result.
func buildURL(result detectionResult, ctx detectionContext) string {
	if result.ForgeType == "" || result.FullName == "" {
		return ""
	}

	// Special case for VS Code local preview mode.
	if result.ForgeType == "vscode" {
		// fullName contains the relative path for VS Code URLs.
		return fmt.Sprintf("/_edit/%s", result.FullName)
	}

	baseURL := strings.TrimSuffix(result.BaseURL, "/")

	// Special case for Bitbucket (preserved from original behavior).
	if strings.Contains(baseURL, "bitbucket.org") {
		return fmt.Sprintf("%s/%s/src/%s/%s?mode=edit", baseURL, result.FullName, ctx.branch, ctx.repoRel)
	}

	return forge.GenerateEditURL(result.ForgeType, baseURL, result.FullName, ctx.branch, ctx.repoRel)
}
