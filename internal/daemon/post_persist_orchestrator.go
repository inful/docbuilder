package daemon

import (
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// PostPersistOrchestrator coordinates all post-persistence operations
type PostPersistOrchestrator interface {
	// ExecutePostPersistStage executes all post-persistence operations in the correct order
	ExecutePostPersistStage(
		report *hugo.BuildReport,
		genErr error,
		context *PostPersistContext,
	) error
}

// PostPersistContext contains all the context needed for post-persistence operations
type PostPersistContext struct {
	DeltaPlan  *DeltaPlan
	Job        *BuildJob
	StateMgr   services.StateManager
	Workspace  string
	OutDir     string
	Config     *config.Config
	Generator  HugoGenerator
	Repos      []config.Repository
	SkipReport *hugo.BuildReport
}

// PostPersistOrchestratorImpl implements PostPersistOrchestrator
type PostPersistOrchestratorImpl struct {
	deltaManager      DeltaManager
	metricsCollector  BuildMetricsCollector
	statePersister    StatePersister
	liveReloadManager LiveReloadManager
}

// NewPostPersistOrchestrator creates a new post-persist orchestrator
func NewPostPersistOrchestrator() PostPersistOrchestrator {
	return &PostPersistOrchestratorImpl{
		deltaManager:      NewDeltaManager(),
		metricsCollector:  NewBuildMetricsCollector(),
		statePersister:    NewStatePersister(),
		liveReloadManager: NewLiveReloadManager(),
	}
}

// ExecutePostPersistStage executes all post-persistence operations in the correct order
func (po *PostPersistOrchestratorImpl) ExecutePostPersistStage(
	report *hugo.BuildReport,
	genErr error,
	context *PostPersistContext,
) error {
	if report == nil {
		return nil
	}

	// Step 1: Attach delta metadata to the report
	po.deltaManager.AttachDeltaMetadata(report, context.DeltaPlan, context.Job)

	// Step 2: Recompute global doc hash for partial builds
	deletionsDetected, err := po.deltaManager.RecomputeGlobalDocHash(
		report,
		context.DeltaPlan,
		context.StateMgr,
		context.Job,
		context.Workspace,
		context.Config,
	)
	if err != nil {
		return fmt.Errorf("recomputing global doc hash: %w", err)
	}

	// Step 3: Record deletion metrics if any were detected
	po.metricsCollector.RecordDeletions(deletionsDetected, context.Job)

	// Step 4: Update repository build metrics and document counts
	err = po.metricsCollector.UpdateRepositoryMetrics(
		context.Repos,
		context.StateMgr,
		context.OutDir,
		genErr,
		context.SkipReport,
	)
	if err != nil {
		return fmt.Errorf("updating repository metrics: %w", err)
	}

	// Step 5: Persist build state (commit heads, config hash, report checksum, doc hash)
	err = po.statePersister.PersistBuildState(
		context.Repos,
		context.StateMgr,
		context.Workspace,
		context.OutDir,
		context.Generator,
		report,
		genErr,
	)
	if err != nil {
		return fmt.Errorf("persisting build state: %w", err)
	}

	// Step 6: Broadcast live reload update
	po.liveReloadManager.BroadcastUpdate(report, context.Job)

	return nil
}
