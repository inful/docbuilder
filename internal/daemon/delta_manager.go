package daemon

import (
	"git.home.luguber.info/inful/docbuilder/internal/build/delta"
	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// deltaManager is kept as a thin compatibility wrapper.
// The canonical implementation lives in internal/build/delta.
type deltaManager struct {
	inner *delta.Manager
}

// NewDeltaManager is kept for backward compatibility.
func NewDeltaManager() *deltaManager { return &deltaManager{inner: delta.NewManager()} }

// AttachDeltaMetadata is kept for backward compatibility with existing daemon tests/callers.
func (dm *deltaManager) AttachDeltaMetadata(report *models.BuildReport, deltaPlan *DeltaPlan, job *BuildJob) {
	var reasons map[string]string
	if job != nil && job.TypedMeta != nil {
		reasons = job.TypedMeta.DeltaRepoReasons
	}
	if dm == nil || dm.inner == nil {
		return
	}
	dm.inner.AttachDeltaMetadata(report, deltaPlan, reasons)
}

// RecomputeGlobalDocHash is kept for backward compatibility with existing daemon tests/callers.
func (dm *deltaManager) RecomputeGlobalDocHash(
	report *models.BuildReport,
	deltaPlan *DeltaPlan,
	meta state.RepositoryMetadataStore,
	job *BuildJob,
	workspace string,
	cfgAny *cfg.Config,
) (int, error) {
	var repos []cfg.Repository
	if job != nil && job.TypedMeta != nil {
		repos = job.TypedMeta.Repositories
	}
	if dm == nil || dm.inner == nil {
		return 0, nil
	}
	return dm.inner.RecomputeGlobalDocHash(report, deltaPlan, meta, repos, workspace, cfgAny)
}
