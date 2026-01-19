package models

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// Generator defines the interface required by stages to interact with the site generator.
type Generator interface {
	Config() *config.Config
	OutputDir() string
	StageDir() string
	Recorder() metrics.Recorder
	StateManager() state.RepositoryMetadataWriter
	ComputeConfigHash() string
	GenerateHugoConfig() error
	BuildRoot() string
	CreateHugoStructure() error
	CopyContentFilesWithState(ctx context.Context, docFiles []docs.DocFile, bs *BuildState) error
	Observer() BuildObserver
	ExistingSiteValidForSkip() bool
	Renderer() Renderer
}

// GitState manages git repository operations and state tracking.
type GitState struct {
	Repositories      []config.Repository
	RepoPaths         map[string]string
	WorkspaceDir      string
	PreHeads          map[string]string
	PostHeads         map[string]string
	CommitDates       map[string]time.Time
	AllReposUnchanged bool
}

func (gs *GitState) SetPreHead(repoName, hash string) {
	if gs.PreHeads == nil {
		gs.PreHeads = make(map[string]string)
	}
	gs.PreHeads[repoName] = hash
}

func (gs *GitState) SetPostHead(repoName, hash string) {
	if gs.PostHeads == nil {
		gs.PostHeads = make(map[string]string)
	}
	gs.PostHeads[repoName] = hash
}

func (gs *GitState) SetCommitDate(repoName string, date time.Time) {
	if gs.CommitDates == nil {
		gs.CommitDates = make(map[string]time.Time)
	}
	gs.CommitDates[repoName] = date
}

func (gs *GitState) GetCommitDate(repoName string) (time.Time, bool) {
	if gs.CommitDates == nil {
		return time.Time{}, false
	}
	date, ok := gs.CommitDates[repoName]
	return date, ok
}

// AllReposUnchangedComputed computes whether all repositories had no HEAD changes.
func (gs *GitState) AllReposUnchangedComputed() bool {
	if len(gs.PreHeads) == 0 {
		return false
	}
	for repo, preHead := range gs.PreHeads {
		if postHead, exists := gs.PostHeads[repo]; !exists || preHead != postHead {
			return false
		}
	}
	return true
}

// DocsState manages documentation discovery and processing state.
type DocsState struct {
	Files          []docs.DocFile
	FilesByRepo    map[string][]docs.DocFile
	FilesBySection map[string][]docs.DocFile
	IsSingleRepo   bool
}

// BuildIndexes populates the repository and section indexes.
func (ds *DocsState) BuildIndexes() {
	if ds.FilesByRepo == nil {
		ds.FilesByRepo = make(map[string][]docs.DocFile)
	}
	if ds.FilesBySection == nil {
		ds.FilesBySection = make(map[string][]docs.DocFile)
	}

	for i := range ds.Files {
		file := &ds.Files[i]
		repoKey := file.Repository
		if file.Forge != "" {
			repoKey = file.Forge + "/" + repoKey
		}
		ds.FilesByRepo[repoKey] = append(ds.FilesByRepo[repoKey], *file)

		sectionKey := repoKey
		if file.Section != "" {
			sectionKey = sectionKey + "/" + file.Section
		}
		ds.FilesBySection[sectionKey] = append(ds.FilesBySection[sectionKey], *file)
	}
}

// PipelineState tracks execution state and metadata across stages.
type PipelineState struct {
	ConfigHash string
	StartTime  time.Time
}

// BuildState carries mutable state and metrics across stages.
type BuildState struct {
	Generator Generator
	Report    *BuildReport

	Git      GitState
	Docs     DocsState
	Pipeline PipelineState
}

// NewBuildState constructs a BuildState with sub-state initialization.
func NewBuildState(g Generator, docFiles []docs.DocFile, report *BuildReport) *BuildState {
	startTime := time.Now()

	repoSet := make(map[string]struct{})
	for i := range docFiles {
		repoSet[docFiles[i].Repository] = struct{}{}
	}
	isSingleRepo := len(repoSet) == 1

	bs := &BuildState{
		Generator: g,
		Report:    report,
		Docs: DocsState{
			Files:        docFiles,
			IsSingleRepo: isSingleRepo,
		},
		Pipeline: PipelineState{
			StartTime: startTime,
		},
		Git: GitState{
			PreHeads:    make(map[string]string),
			PostHeads:   make(map[string]string),
			CommitDates: make(map[string]time.Time),
		},
	}

	if len(docFiles) > 0 {
		bs.Docs.BuildIndexes()
	}

	return bs
}
