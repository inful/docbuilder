package transforms

import (
	"path/filepath"
	"regexp"
	"strings"
)

// rewriteImagePathsTransform rewrites relative image paths in markdown to work with Hugo's
// page bundle structure. When a markdown file references images like ./images/file.png,
// and the file becomes a page bundle (e.g., /repo/file/index.html), the relative path
// breaks. This transform adjusts paths to be relative to the repository/section root.
type rewriteImagePathsTransform struct{}

func (t rewriteImagePathsTransform) Name() string { return "rewrite_image_paths" }

func (t rewriteImagePathsTransform) Stage() TransformStage {
	return StageBuild
}

func (t rewriteImagePathsTransform) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{},
		MustRunBefore:               []string{},
		RequiresOriginalFrontMatter: false,
		ModifiesContent:             true,
		ModifiesFrontMatter:         false,
		RequiresConfig:              false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           false,
		RequiresEditLinkResolver:    false,
		RequiresFileMetadata:        false,
	}
}

func (t rewriteImagePathsTransform) Transform(p PageAdapter) error {
	pg, ok := p.(*PageShim)
	if !ok {
		return nil
	}

	// Skip if no relative image references
	if !strings.Contains(pg.Content, "](./") && !strings.Contains(pg.Content, "](../") {
		return nil
	}

	// Rewrite image paths
	pg.Content = t.rewriteImagePaths(pg.Content, pg.Doc.Name)

	return nil
}

// rewriteImagePaths rewrites relative image/asset paths in markdown content.
// Hugo creates page bundles where each page gets its own directory, so relative
// paths like ./images/file.png need to be adjusted to ../images/file.png to
// reference assets in the parent (section/repository) directory.
func (t rewriteImagePathsTransform) rewriteImagePaths(content, fileName string) string {
	// Pattern to match markdown image syntax: ![alt](path)
	// Captures:
	// - Group 1: Everything before the path (![alt text]()
	// - Group 2: The relative path starting with ./ or ../
	// - Group 3: Everything after the path ()
	imagePattern := regexp.MustCompile(`(!\[[^\]]*\]\()(\./[^)]+|\.\./[^)]+)(\))`)

	// Only rewrite for non-index files (index files don't become page bundles)
	isIndex := strings.ToLower(fileName) == "index" || strings.ToLower(fileName) == "readme"
	if isIndex {
		return content
	}

	return imagePattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := imagePattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		prefix := parts[1] // ![alt](
		path := parts[2]   // ./images/file.png or ../images/file.png
		suffix := parts[3] // )

		// Adjust relative paths
		// ./images/file.png -> ../images/file.png (go up one level from page bundle)
		if strings.HasPrefix(path, "./") {
			path = ".." + path[1:]
		}
		// ../images/file.png -> ../../images/file.png (go up one more level)
		// This handles cases where the image is referenced from a subsection
		// Note: This might need adjustment based on actual structure

		// Check if the path points to an asset (by extension)
		ext := strings.ToLower(filepath.Ext(path))
		isAsset := isAssetExtension(ext)

		// Only rewrite asset paths
		if !isAsset {
			return match
		}

		// Lowercase the filename portion to match GetHugoPath() behavior in discovery.go
		// which lowercases all filenames (line 241: filename := strings.ToLower(df.Name))
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		base = strings.ToLower(base)
		if dir == "." {
			path = base
		} else {
			path = filepath.Join(dir, base)
		}

		return prefix + path + suffix
	})
}

// isAssetExtension checks if a file extension is an asset type
func isAssetExtension(ext string) bool {
	assetExtensions := []string{
		// Images
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico",
		// Documents
		".pdf",
		// Video
		".mp4", ".webm", ".ogv",
		// Other
		".csv", ".json", ".yaml", ".yml", ".xml",
	}
	for _, assetExt := range assetExtensions {
		if ext == assetExt {
			return true
		}
	}
	return false
}

func init() {
	Register(rewriteImagePathsTransform{})
}
