package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/services"
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

// buildStateManager interface for persisting build state
type buildStateManager interface {
	SetRepoLastCommit(string, string, string, string)
	SetLastConfigHash(string)
	SetLastReportChecksum(string)
	SetLastGlobalDocFilesHash(string)
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
	sm, ok := stateMgr.(buildStateManager)
	if !ok || sm == nil {
		return nil //nolint:nilerr // Skip if state manager unavailable
	}
	if genErr != nil {
		return genErr
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
	sm buildStateManager,
) error {
	for _, repo := range repos {
		repoPath := filepath.Join(workspace, repo.Name)
		head, err := hugoReadRepoHead(repoPath)
		if err == nil && head != "" {
			sm.SetRepoLastCommit(repo.URL, repo.Name, repo.Branch, head)
		}
		// Continue with other repos even if one fails
	}
	return nil
}

// persistConfigHash persists the Hugo configuration hash
// nolint:unparam // These helpers currently never return an error.
func (sp *StatePersisterImpl) persistConfigHash(
	generator HugoGenerator,
	sm buildStateManager,
) error {
	if hash := generator.ComputeConfigHashForPersistence(); hash != "" {
		sm.SetLastConfigHash(hash)
	}
	return nil
}

// persistReportChecksum persists the build report checksum
// nolint:unparam // These helpers currently never return an error.
func (sp *StatePersisterImpl) persistReportChecksum(
	outDir string,
	sm buildStateManager,
) error {
	reportPath := filepath.Join(outDir, "build-report.json")
	brData, err := os.ReadFile(reportPath)
	if err != nil {
		return nil //nolint:nilerr // Skip if report file doesn't exist
	}

	sum := sha256.Sum256(brData)
	checksum := hex.EncodeToString(sum[:])
	sm.SetLastReportChecksum(checksum)

	return nil
}

// persistGlobalDocFilesHash persists the global documentation files hash
// nolint:unparam // These helpers currently never return an error.
func (sp *StatePersisterImpl) persistGlobalDocFilesHash(
	report *hugo.BuildReport,
	sm buildStateManager,
) error {
	if report.DocFilesHash != "" {
		sm.SetLastGlobalDocFilesHash(report.DocFilesHash)
	}
	return nil
}
