package hugo

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// BuildState carries mutable state and metrics across stages. Extracted from stages.go (Phase 1 refactor).
type BuildState struct {
	Generator         *Generator
	Docs              []docs.DocFile
	Report            *BuildReport
	start             time.Time
	Repositories      []config.Repository // configured repositories (post-filter)
	RepoPaths         map[string]string   // name -> local filesystem path
	WorkspaceDir      string              // root workspace for git operations
	preHeads          map[string]string   // repo -> head before update (for existing repos)
	postHeads         map[string]string   // repo -> head after clone/update
	AllReposUnchanged bool                // set true if every repo head unchanged (and no fresh clones)
	ConfigHash        string              // fingerprint of relevant config used for change detection
}

// newBuildState constructs a BuildState.
func newBuildState(g *Generator, docFiles []docs.DocFile, report *BuildReport) *BuildState {
	return &BuildState{Generator: g, Docs: docFiles, Report: report, start: time.Now()}
}
