package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/services"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// StatePersister handles persisting build state and metadata
type StatePersister interface {
	// PersistBuildState persists commit heads, config hash, report checksum, and global doc hash
	PersistBuildState(
		repos []config.Repository,
		stateMgr services.StateManager,
		workspace string,
		outDir string,
		generator HugoGenerator,
		report *hugo.BuildReport,
		genErr error,
	) error
}

// StatePersisterImpl implements StatePersister
type StatePersisterImpl struct{}

// NewStatePersister creates a new state persister
func NewStatePersister() StatePersister {
	return &StatePersisterImpl{}
}

// The buildStateManager interface was removed - using individual state interfaces

// stateManager wraps multiple state interfaces for cleaner passing
type stateManager struct {
	configStore state.ConfigurationStateStore
	repoInit    state.RepositoryInitializer
	repoCommit  state.RepositoryCommitTracker
}

// HugoGenerator interface for getting config hash
type HugoGenerator interface {
	ComputeConfigHashForPersistence() string
}

// PersistBuildState persists commit heads, config hash, report checksum, and global doc hash
func (sp *StatePersisterImpl) PersistBuildState(
	repos []config.Repository,
	stateMgr services.StateManager,
	workspace string,
	outDir string,
	generator HugoGenerator,
	report *hugo.BuildReport,
	genErr error,
) error {
	// Type assert to needed interfaces
	configStore, hasConfigStore := stateMgr.(state.ConfigurationStateStore)
	repoInit, hasRepoInit := stateMgr.(state.RepositoryInitializer)
	repoCommit, hasRepoCommit := stateMgr.(state.RepositoryCommitTracker)

	if !hasConfigStore && !hasRepoInit && !hasRepoCommit {
		return nil //nolint:nilerr // Skip if state manager unavailable
	}
	if genErr != nil {
		return genErr
	}

	sm := &stateManager{
		configStore: configStore,
		repoInit:    repoInit,
		repoCommit:  repoCommit,
	}

	// Persist repository commit heads
	if err := sp.persistRepositoryCommits(repos, workspace, sm); err != nil {
		return fmt.Errorf("persisting repository commits: %w", err)
	}

	// Persist configuration hash
	if err := sp.persistConfigHash(generator, sm); err != nil {
		return fmt.Errorf("persisting config hash: %w", err)
	}

	// Persist build report checksum
	if err := sp.persistReportChecksum(outDir, sm); err != nil {
		return fmt.Errorf("persisting report checksum: %w", err)
	}

	// Persist global document files hash
	if err := sp.persistGlobalDocFilesHash(report, sm); err != nil {
		return fmt.Errorf("persisting global doc files hash: %w", err)
	}

	return nil
}

// persistRepositoryCommits persists the latest commit hash for each repository
// nolint:unparam // These helpers currently never return an error.
func (sp *StatePersisterImpl) persistRepositoryCommits(
	repos []config.Repository,
	workspace string,
	sm *stateManager,
) error {
	if sm.repoInit == nil || sm.repoCommit == nil {
		return nil
	}
	for _, repo := range repos {
		sm.repoInit.EnsureRepositoryState(repo.URL, repo.Name, repo.Branch)
		repoPath := filepath.Join(workspace, repo.Name)
		head, err := hugoReadRepoHead(repoPath)
		if err == nil && head != "" {
			sm.repoCommit.SetRepoLastCommit(repo.URL, repo.Name, repo.Branch, head)
		}
		// Continue with other repos even if one fails
	}
	return nil
}

// persistConfigHash persists the Hugo configuration hash
// nolint:unparam // These helpers currently never return an error.
func (sp *StatePersisterImpl) persistConfigHash(
	generator HugoGenerator,
	sm *stateManager,
) error {
	if sm.configStore == nil {
		return nil
	}
	if hash := generator.ComputeConfigHashForPersistence(); hash != "" {
		sm.configStore.SetLastConfigHash(hash)
	}
	return nil
}

// persistReportChecksum persists the build report checksum
// nolint:unparam // These helpers currently never return an error.
func (sp *StatePersisterImpl) persistReportChecksum(
	outDir string,
	sm *stateManager,
) error {
	if sm.configStore == nil {
		return nil
	}
	reportPath := filepath.Join(outDir, "build-report.json")
	brData, err := os.ReadFile(reportPath)
	if err != nil {
		return nil //nolint:nilerr // Skip if report file doesn't exist
	}

	sum := sha256.Sum256(brData)
	checksum := hex.EncodeToString(sum[:])
	sm.configStore.SetLastReportChecksum(checksum)

	return nil
}

// persistGlobalDocFilesHash persists the global documentation files hash
// nolint:unparam // These helpers currently never return an error.
func (sp *StatePersisterImpl) persistGlobalDocFilesHash(
	report *hugo.BuildReport,
	sm *stateManager,
) error {
	if sm.configStore == nil {
		return nil
	}
	if report.DocFilesHash != "" {
		sm.configStore.SetLastGlobalDocFilesHash(report.DocFilesHash)
	}
	return nil
}

// hugoReadRepoHead reads the HEAD commit hash from a git repository.
// Deprecated: Use git.ReadRepoHead directly.
func hugoReadRepoHead(repoPath string) (string, error) {
	return git.ReadRepoHead(repoPath)
}
