package pipeline

import (
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

const indexFileSuffix = "_index"

// Document represents a file being processed through the content pipeline.
// This is the new unified representation that replaces the Page + PageShim + PageAdapter pattern.
type Document struct {
	// Content is the markdown body (transformed in-place by transforms)
	Content string

	// FrontMatter is the YAML front matter (modified directly by transforms)
	FrontMatter map[string]any

	// OriginalFrontMatter preserves the initial front matter for reference
	// (some transforms need to know what was in the original file)
	OriginalFrontMatter map[string]any

	// HadFrontMatter indicates if the original file contained front matter
	HadFrontMatter bool

	// Metadata for transforms to use (read-only during transform phase)
	Path            string    // Hugo content path (e.g., "repo-name/section/file.md")
	IsIndex         bool      // True if this is _index.md or README.md
	Repository      string    // Source repository name
	Forge           string    // Optional forge namespace
	Section         string    // Documentation section
	IsSingleRepo    bool      // True if this is a single-repository build (skip repo namespace in links)
	IsPreviewMode   bool      // True if running in preview/daemon mode
	VSCodeEditLinks bool      // True if VS Code edit links are enabled (via --vscode flag)
	EditURLBase     string    // Base URL override for edit links (from --edit-url-base flag)
	SourceCommit    string    // Git commit SHA
	CommitDate      time.Time // Git commit date
	SourceURL       string    // Repository URL for edit links
	SourceBranch    string    // Git branch name
	Generated       bool      // True if this was generated (not discovered)

	// Internal fields (used by pipeline, not by transforms)
	FilePath     string // Absolute path to source file (for discovered docs)
	RelativePath string // Path relative to repository root (for edit links)
	Extension    string // File extension
	DocsBase     string // Configured docs base path
	Name         string // File name without extension

	// Raw is the serialized output (front matter + content)
	// Set by Serialize transform at the end of pipeline
	Raw []byte
}

// NewDocumentFromDocFile creates a Document from a discovered DocFile.
func NewDocumentFromDocFile(file docs.DocFile, isSingleRepo bool, isPreviewMode bool, vscodeEditLinks bool, editURLBase string) *Document {
	// Determine if this is an index file
	isIndex := isIndexFileName(file.Name)

	return &Document{
		Content:             string(file.Content),
		FrontMatter:         make(map[string]any),
		OriginalFrontMatter: nil, // Will be populated by front matter parser
		HadFrontMatter:      false,
		Path:                file.GetHugoPath(isSingleRepo),
		IsIndex:             isIndex,
		Repository:          file.Repository,
		Forge:               file.Forge,
		Section:             file.Section,
		IsSingleRepo:        isSingleRepo,
		IsPreviewMode:       isPreviewMode,
		VSCodeEditLinks:     vscodeEditLinks,
		EditURLBase:         editURLBase,
		SourceCommit:        "", // Will be set by repository metadata injector
		SourceURL:           "", // Will be set by repository metadata injector
		SourceBranch:        "", // Will be set by repository metadata injector
		Generated:           false,
		FilePath:            file.Path,
		RelativePath:        file.RelativePath,
		Extension:           file.Extension,
		DocsBase:            file.DocsBase,
		Name:                file.Name,
		Raw:                 nil,
	}
}

// isIndexFileName checks if a file name represents an index file.
func isIndexFileName(name string) bool {
	lowerName := strings.ToLower(name)
	return lowerName == "index" || lowerName == "readme" || lowerName == indexFileSuffix
}

// FileTransform modifies a document in the pipeline.
// Can optionally return new documents to inject into the pipeline.
// New documents will be queued and processed through ALL transforms from the beginning.
//
// Transforms should:
// - Modify doc in-place (Content, FrontMatter fields)
// - Return nil error on success
// - Return new documents if generating content based on doc analysis
//
// Important: Generated documents (doc.Generated == true) MUST NOT create new documents.
// The pipeline will automatically reject this to prevent infinite loops.
type FileTransform func(doc *Document) ([]*Document, error)

// FileGenerator creates new documents based on analysis of discovered documents.
// Generators run BEFORE transforms to create missing files (e.g., _index.md).
//
// Generators should:
// - Analyze ctx.Discovered to find gaps (missing index files, etc.)
// - Create new Document instances with Generated = true
// - NOT modify documents in ctx.Discovered (read-only access).
type FileGenerator func(ctx *GenerationContext) ([]*Document, error)

// GenerationContext provides access to discovered files for analysis.
type GenerationContext struct {
	// Discovered contains all files found in repositories (read-only)
	Discovered []*Document

	// Config provides access to build configuration
	Config *config.Config

	// RepositoryMetadata maps repository name to metadata (commit, URL, etc.)
	// Populated during discovery phase
	RepositoryMetadata map[string]RepositoryInfo

	// IsSingleRepo indicates if this is a single-repository build
	// When true, repository namespaces are omitted from paths
	IsSingleRepo bool
}

// RepositoryInfo contains metadata about a repository for use in generation.
type RepositoryInfo struct {
	Name       string
	URL        string
	Commit     string
	CommitDate time.Time
	Branch     string
	Forge      string
	Tags       map[string]string
	DocsBase   string
	DocsPaths  []string // All configured documentation paths
	Namespace  string   // For namespaced repos
}
