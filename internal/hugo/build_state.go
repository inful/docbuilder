package hugo

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// GitState manages git repository operations and state tracking
type GitState struct {
	Repositories      []config.Repository // configured repositories (post-filter)
	RepoPaths         map[string]string   // name -> local filesystem path
	WorkspaceDir      string              // root workspace for git operations
	preHeads          map[string]string   // repo -> head before update (for existing repos)
	postHeads         map[string]string   // repo -> head after clone/update
	AllReposUnchanged bool                // computed lazily: true if every repo head unchanged (and no fresh clones)
}

// AllReposUnchangedComputed computes whether all repositories had no HEAD changes
func (gs *GitState) AllReposUnchangedComputed() bool {
	if len(gs.preHeads) == 0 {
		return false // fresh clones, not unchanged
	}
	for repo, preHead := range gs.preHeads {
		if postHead, exists := gs.postHeads[repo]; !exists || preHead != postHead {
			return false
		}
	}
	return true
}

// SetPreHead records the HEAD commit before update operations
func (gs *GitState) SetPreHead(repo, head string) {
	gs.preHeads[repo] = head
}

// SetPostHead records the HEAD commit after clone/update operations
func (gs *GitState) SetPostHead(repo, head string) {
	gs.postHeads[repo] = head
}

// DocsState manages documentation discovery and processing state
type DocsState struct {
	Files          []docs.DocFile            // discovered documentation files
	FilesByRepo    map[string][]docs.DocFile // files grouped by repository (computed lazily)
	FilesBySection map[string][]docs.DocFile // files grouped by section (computed lazily)
}

// BuildIndexes populates the repository and section indexes
func (ds *DocsState) BuildIndexes() {
	if ds.FilesByRepo == nil {
		ds.FilesByRepo = make(map[string][]docs.DocFile)
	}
	if ds.FilesBySection == nil {
		ds.FilesBySection = make(map[string][]docs.DocFile)
	}

	for _, file := range ds.Files {
		// Repository index
		repoKey := file.Repository
		if file.Forge != "" {
			repoKey = file.Forge + "/" + repoKey
		}
		ds.FilesByRepo[repoKey] = append(ds.FilesByRepo[repoKey], file)

		// Section index
		sectionKey := repoKey
		if file.Section != "" {
			sectionKey = sectionKey + "/" + file.Section
		}
		ds.FilesBySection[sectionKey] = append(ds.FilesBySection[sectionKey], file)
	}
}

// PipelineState tracks execution state and metadata across stages
type PipelineState struct {
	ConfigHash string    // fingerprint of relevant config for change detection
	StartTime  time.Time // pipeline start time
}

// BuildState carries mutable state and metrics across stages.
// Decomposed into sub-states for better organization (Phase 5 refactor).
type BuildState struct {
	Generator *Generator
	Report    *BuildReport

	// Sub-state components
	Git      GitState
	Docs     DocsState
	Pipeline PipelineState

	// Legacy field mirrors (kept for temporary backward compatibility)
	start             time.Time           // use Pipeline.StartTime instead
	Repositories      []config.Repository // use Git.Repositories instead
	RepoPaths         map[string]string   // use Git.RepoPaths instead
	WorkspaceDir      string              // use Git.WorkspaceDir instead
	preHeads          map[string]string   // prefer Git methods
	postHeads         map[string]string   // prefer Git methods
	AllReposUnchanged bool                // prefer Git.AllReposUnchangedComputed()
	ConfigHash        string              // use Pipeline.ConfigHash instead
}

// Legacy accessors for backward compatibility
func (bs *BuildState) SyncLegacyFields() {
	// Keep legacy fields in sync with sub-states
	bs.start = bs.Pipeline.StartTime
	bs.Repositories = bs.Git.Repositories
	bs.RepoPaths = bs.Git.RepoPaths
	bs.WorkspaceDir = bs.Git.WorkspaceDir
	bs.AllReposUnchanged = bs.Git.AllReposUnchanged
	bs.ConfigHash = bs.Pipeline.ConfigHash
	bs.preHeads = bs.Git.preHeads
	bs.postHeads = bs.Git.postHeads
}

// newBuildState constructs a BuildState with sub-state initialization.
func newBuildState(g *Generator, docFiles []docs.DocFile, report *BuildReport) *BuildState {
	startTime := time.Now()
	bs := &BuildState{
		Generator: g,
		Report:    report,
		Docs: DocsState{
			Files: docFiles,
		},
		Pipeline: PipelineState{
			StartTime: startTime,
		},
		Git: GitState{
			preHeads:  make(map[string]string),
			postHeads: make(map[string]string),
		},
		// Legacy field initialization for backward compatibility
		start: startTime,
	}

	// Initialize docs indexes if we have files
	if len(docFiles) > 0 {
		bs.Docs.BuildIndexes()
	}

	return bs
}
