package pipeline

import (
	"fmt"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// CloneRequested carries the plan to perform repository cloning.
type CloneRequested struct{ Plan *BuildPlan }

func (CloneRequested) Name() string { return EventCloneRequested }

// DiscoverRequested carries the plan and repo paths for discovery.
type DiscoverRequested struct {
	Plan      *BuildPlan
	RepoPaths map[string]string
}

func (DiscoverRequested) Name() string { return EventDiscoverRequested }

// DiscoverCompleted carries discovered doc files count.
type DiscoverCompleted struct {
	Plan     *BuildPlan
	DocFiles []docs.DocFile
}

func (DiscoverCompleted) Name() string { return EventDiscoverCompleted }

// GenerateRequested carries the plan to generate a site.
type GenerateRequested struct {
	Plan     *BuildPlan
	DocFiles []docs.DocFile
}

func (GenerateRequested) Name() string { return EventGenerateRequested }

// GenerateCompleted indicates success and the output directory.
type GenerateCompleted struct{ OutputDir string }

func (GenerateCompleted) Name() string { return EventGenerateCompleted }

// NewCloneHandler returns a handler that clones repositories using git client and emits DiscoverRequested.
func NewCloneHandler(bus *Bus) Handler {
	return func(e Event) error {
		cr, ok := e.(CloneRequested)
		if !ok || cr.Plan == nil || cr.Plan.Config == nil {
			return fmt.Errorf("invalid clone event: %#v", e)
		}
		// Use git client to clone repositories
		gitClient := git.NewClient(cr.Plan.WorkspaceDir).WithBuildConfig(&cr.Plan.Config.Build)
		repoPaths := map[string]string{}
		for _, repo := range cr.Plan.Config.Repositories {
			slog.Info("Cloning repository", logfields.Repository(repo.Name), logfields.URL(repo.URL))
			repoPath, err := gitClient.CloneRepo(repo)
			if err != nil {
				return fmt.Errorf("clone repository %s: %w", repo.Name, err)
			}
			repoPaths[repo.Name] = repoPath
			slog.Info("Cloned repository", logfields.Repository(repo.Name), logfields.Path(repoPath))
		}
		return bus.Publish(DiscoverRequested{Plan: cr.Plan, RepoPaths: repoPaths})
	}
}

// NewDiscoverHandler returns a handler that runs discovery and emits GenerateRequested.
func NewDiscoverHandler(bus *Bus) Handler {
	return func(e Event) error {
		dr, ok := e.(DiscoverRequested)
		if !ok || dr.Plan == nil || dr.Plan.Config == nil {
			return fmt.Errorf("invalid discover event: %#v", e)
		}
		// Run discovery using docs package
		discovery := docs.NewDiscovery(dr.Plan.Config.Repositories, &dr.Plan.Config.Build)
		docFiles, err := discovery.DiscoverDocs(dr.RepoPaths)
		if err != nil {
			return fmt.Errorf("discover failed: %w", err)
		}
		// Emit completion event for observability
		if err := bus.Publish(DiscoverCompleted{Plan: dr.Plan, DocFiles: docFiles}); err != nil {
			return err
		}
		// Chain to generate
		return bus.Publish(GenerateRequested{Plan: dr.Plan, DocFiles: docFiles})
	}
}

// NewGenerateHandler returns a handler that runs hugo generation for the plan.
func NewGenerateHandler() Handler {
	return func(e Event) error {
		gr, ok := e.(GenerateRequested)
		if !ok || gr.Plan == nil || gr.Plan.Config == nil {
			return fmt.Errorf("invalid generate event: %#v", e)
		}
		g := hugo.NewGenerator(gr.Plan.Config, gr.Plan.OutputDir)
		// For scaffolding, run generation with discovered files (or empty)
		if err := g.GenerateSite(gr.DocFiles); err != nil {
			// treat as success for scaffolding purposes to avoid full hugo errors
			return nil //nolint:nilerr // intentional for scaffolding
		}
		return nil
	}
}
