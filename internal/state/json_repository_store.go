package state

import (
	"context"
	"sort"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// jsonRepositoryStore implements RepositoryStore for the JSON store.
type jsonRepositoryStore struct {
	store *JSONStore
}

func (rs *jsonRepositoryStore) Create(_ context.Context, repo *Repository) foundation.Result[*Repository, error] {
	if repo == nil {
		return foundation.Err[*Repository, error](
			errors.ValidationError("repository cannot be nil").Build(),
		)
	}

	// Validate the repository
	if validationResult := repo.Validate(); !validationResult.Valid {
		return foundation.Err[*Repository, error](validationResult.ToError())
	}

	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	// Check if repository already exists
	if _, exists := rs.store.repositories[repo.URL]; exists {
		return foundation.Err[*Repository, error](
			errors.ValidationError("repository already exists").
				WithContext("url", repo.URL).
				Build(),
		)
	}

	// Set timestamps
	now := time.Now()
	repo.CreatedAt = now
	repo.UpdatedAt = now

	// Store the repository
	rs.store.repositories[repo.URL] = repo

	// Auto-save if enabled
	// The file is written when the state is saved: a write for every
	// change rewrote the whole file hundreds of times per build.
	rs.store.dirty = true

	return foundation.Ok[*Repository, error](repo)
}

func (rs *jsonRepositoryStore) GetByURL(_ context.Context, url string) foundation.Result[foundation.Option[*Repository], error] {
	if url == "" {
		return foundation.Err[foundation.Option[*Repository], error](
			errors.ValidationError("URL cannot be empty").Build(),
		)
	}

	rs.store.mu.RLock()
	defer rs.store.mu.RUnlock()

	if repo, exists := rs.store.repositories[url]; exists {
		// Return a copy to prevent external modification
		repoCopy := *repo
		return foundation.Ok[foundation.Option[*Repository], error](foundation.Some(&repoCopy))
	}

	return foundation.Ok[foundation.Option[*Repository], error](foundation.None[*Repository]())
}

func (rs *jsonRepositoryStore) Update(_ context.Context, repo *Repository) foundation.Result[*Repository, error] {
	return updateValidatableEntity[Repository](
		rs.store,
		"repository",
		repo,
		func() bool { _, ok := rs.store.repositories[repo.URL]; return ok },
		func() { repo.UpdatedAt = time.Now() },
		func() { rs.store.repositories[repo.URL] = repo },
		func() foundation.Result[*Repository, error] {
			return foundation.Err[*Repository, error](
				errors.NotFoundError("repository").
					WithContext("url", repo.URL).
					Build(),
			)
		},
	)
}

func (rs *jsonRepositoryStore) List(_ context.Context) foundation.Result[[]Repository, error] {
	rs.store.mu.RLock()
	defer rs.store.mu.RUnlock()

	repositories := make([]Repository, 0, len(rs.store.repositories))
	for _, repo := range rs.store.repositories {
		repositories = append(repositories, *repo)
	}

	// Sort by name for consistent ordering
	sort.Slice(repositories, func(i, j int) bool {
		return repositories[i].Name < repositories[j].Name
	})

	return foundation.Ok[[]Repository, error](repositories)
}

func (rs *jsonRepositoryStore) Delete(_ context.Context, url string) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	return deleteEntity(
		url,
		func() bool { _, exists := rs.store.repositories[url]; return exists },
		func() { delete(rs.store.repositories, url) },
		"repository",
		func() { rs.store.dirty = true },
	)
}

func (rs *jsonRepositoryStore) IncrementBuildCount(_ context.Context, url string, success bool) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			errors.NotFoundError("repository").
				WithContext("url", url).
				Build(),
		)
	}

	// Update counters
	now := time.Now()
	repo.LastBuild = foundation.Some(now)
	repo.BuildCount++
	if !success {
		repo.ErrorCount++
	}
	repo.UpdatedAt = now

	// Auto-save if enabled
	rs.store.dirty = true

	return foundation.Ok[struct{}, error](struct{}{})
}

// SetLastCommit records the commit a repository was built at.
//
// The entry is changed under the lock of the store: a read of the
// entry, a change of the copy and a write of it back lose the changes
// that another writer made in between.
func (rs *jsonRepositoryStore) SetLastCommit(_ context.Context, url, commit string, updatedAt time.Time) foundation.Result[struct{}, error] {
	if url == "" || commit == "" {
		return foundation.Err[struct{}, error](
			errors.ValidationError("url and commit are required").Build(),
		)
	}

	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			errors.NotFoundError("repository").
				WithContext("url", url).
				Build(),
		)
	}

	repo.LastCommit = foundation.Some(commit)
	repo.UpdatedAt = updatedAt

	rs.store.dirty = true

	return foundation.Ok[struct{}, error](struct{}{})
}

// SetLastDiscovery records the time of the last discovery of a
// repository, under the lock of the store.
func (rs *jsonRepositoryStore) SetLastDiscovery(_ context.Context, url string, at time.Time) foundation.Result[struct{}, error] {
	if url == "" {
		return foundation.Err[struct{}, error](
			errors.ValidationError("url is required").Build(),
		)
	}

	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			errors.NotFoundError("repository").
				WithContext("url", url).
				Build(),
		)
	}

	repo.LastDiscovery = foundation.Some(at)
	repo.UpdatedAt = at

	rs.store.dirty = true

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) SetDocumentCount(_ context.Context, url string, count int) foundation.Result[struct{}, error] {
	if count < 0 {
		return foundation.Err[struct{}, error](
			errors.ValidationError("document count cannot be negative").Build(),
		)
	}

	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			errors.NotFoundError("repository").
				WithContext("url", url).
				Build(),
		)
	}

	repo.DocumentCount = count
	repo.UpdatedAt = time.Now()

	// Auto-save if enabled
	rs.store.dirty = true

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) SetDocFilesHash(_ context.Context, url, hash string) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			errors.NotFoundError("repository").
				WithContext("url", url).
				Build(),
		)
	}

	repo.DocFilesHash = foundation.Some(hash)
	repo.UpdatedAt = time.Now()

	// Auto-save if enabled
	rs.store.dirty = true

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) SetDocFilePaths(_ context.Context, url string, paths []string) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			errors.NotFoundError("repository").
				WithContext("url", url).
				Build(),
		)
	}

	// Make a copy of the paths to prevent external modification
	repo.DocFilePaths = append([]string{}, paths...)
	repo.UpdatedAt = time.Now()

	// Auto-save if enabled
	rs.store.dirty = true

	return foundation.Ok[struct{}, error](struct{}{})
}
