package state

import (
	"context"
	"sort"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// jsonRepositoryStore implements RepositoryStore for the JSON store.
type jsonRepositoryStore struct {
	store *JSONStore
}

func (rs *jsonRepositoryStore) Create(_ context.Context, repo *Repository) foundation.Result[*Repository, error] {
	if repo == nil {
		return foundation.Err[*Repository, error](
			foundation.ValidationError("repository cannot be nil").Build(),
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
			foundation.ValidationError("repository already exists").
				WithContext(foundation.Fields{"url": repo.URL}).
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
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			// Remove from memory if save failed
			delete(rs.store.repositories, repo.URL)
			return foundation.Err[*Repository, error](
				foundation.InternalError("failed to save repository").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[*Repository, error](repo)
}

func (rs *jsonRepositoryStore) GetByURL(_ context.Context, url string) foundation.Result[foundation.Option[*Repository], error] {
	if url == "" {
		return foundation.Err[foundation.Option[*Repository], error](
			foundation.ValidationError("URL cannot be empty").Build(),
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
	if repo == nil {
		return foundation.Err[*Repository, error](
			foundation.ValidationError("repository cannot be nil").Build(),
		)
	}

	// Validate the repository
	if validationResult := repo.Validate(); !validationResult.Valid {
		return foundation.Err[*Repository, error](validationResult.ToError())
	}

	return updateEntity[Repository](
		rs.store,
		repo,
		func() bool { _, ok := rs.store.repositories[repo.URL]; return ok },
		func() { repo.UpdatedAt = time.Now() },
		func() { rs.store.repositories[repo.URL] = repo },
		func() foundation.Result[*Repository, error] {
			return foundation.Err[*Repository, error](
				foundation.NotFoundError("repository").
					WithContext(foundation.Fields{"url": repo.URL}).
					Build(),
			)
		},
		"failed to save repository update",
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
		func() error {
			if rs.store.autoSaveEnabled {
				return rs.store.saveToDiskUnsafe()
			}
			return nil
		},
		"repository",
		"failed to save repository deletion",
	)
}

func (rs *jsonRepositoryStore) IncrementBuildCount(_ context.Context, url string, success bool) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		// Create repository if it doesn't exist (same behavior as original)
		name := url
		if slash := len(url) - 1; slash >= 0 {
			for i := slash; i >= 0; i-- {
				if url[i] == '/' {
					name = url[i+1:]
					break
				}
			}
		}
		if name != url && len(name) > 4 && name[len(name)-4:] == ".git" {
			name = name[:len(name)-4]
		}

		repo = &Repository{
			URL:       url,
			Name:      name,
			Branch:    "main", // default
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		rs.store.repositories[url] = repo
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
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save build count update").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) SetDocumentCount(_ context.Context, url string, count int) foundation.Result[struct{}, error] {
	if count < 0 {
		return foundation.Err[struct{}, error](
			foundation.ValidationError("document count cannot be negative").Build(),
		)
	}

	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		// Create repository if it doesn't exist
		name := url
		if slash := len(url) - 1; slash >= 0 {
			for i := slash; i >= 0; i-- {
				if url[i] == '/' {
					name = url[i+1:]
					break
				}
			}
		}
		if name != url && len(name) > 4 && name[len(name)-4:] == ".git" {
			name = name[:len(name)-4]
		}

		repo = &Repository{
			URL:       url,
			Name:      name,
			Branch:    "main",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		rs.store.repositories[url] = repo
	}

	repo.DocumentCount = count
	repo.UpdatedAt = time.Now()

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save document count update").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) SetDocFilesHash(_ context.Context, url, hash string) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("repository").
				WithContext(foundation.Fields{"url": url}).
				Build(),
		)
	}

	repo.DocFilesHash = foundation.Some(hash)
	repo.UpdatedAt = time.Now()

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save doc files hash update").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}

func (rs *jsonRepositoryStore) SetDocFilePaths(_ context.Context, url string, paths []string) foundation.Result[struct{}, error] {
	rs.store.mu.Lock()
	defer rs.store.mu.Unlock()

	repo, exists := rs.store.repositories[url]
	if !exists {
		return foundation.Err[struct{}, error](
			foundation.NotFoundError("repository").
				WithContext(foundation.Fields{"url": url}).
				Build(),
		)
	}

	// Make a copy of the paths to prevent external modification
	repo.DocFilePaths = append([]string{}, paths...)
	repo.UpdatedAt = time.Now()

	// Auto-save if enabled
	if rs.store.autoSaveEnabled {
		if err := rs.store.saveToDiskUnsafe(); err != nil {
			return foundation.Err[struct{}, error](
				foundation.InternalError("failed to save doc file paths update").
					WithCause(err).
					Build(),
			)
		}
	}

	return foundation.Ok[struct{}, error](struct{}{})
}
