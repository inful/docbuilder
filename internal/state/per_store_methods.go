package state

// per_store_methods.go inlines the per-store methods (previously on
// jsonRepositoryStore, jsonConfigurationStore, jsonDaemonInfoStore
// wrappers) as methods on *JSONStore. The per-store interfaces have
// been removed from interfaces.go; *JSONStore IS the store.

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// void is a typed nil for Result return values.
type void = struct{}

// --- Repository operations (per-store interface inlined) ---

// RepositoryCreate creates a new repository record.
func (js *JSONStore) RepositoryCreate(_ context.Context, repo *Repository) foundation.Result[*Repository, error] {
	if repo == nil {
		return foundation.Err[*Repository, error](errors.New("repository cannot be nil"))
	}
	if validationResult := repo.Validate(); !validationResult.Valid {
		return foundation.Err[*Repository, error](validationResult.ToError())
	}

	js.mu.Lock()
	defer js.mu.Unlock()

	if _, exists := js.repositories[repo.URL]; exists {
		return foundation.Err[*Repository, error](fmt.Errorf("repository already exists: %s", repo.URL))
	}

	now := time.Now()
	repo.CreatedAt = now
	repo.UpdatedAt = now
	js.repositories[repo.URL] = repo

	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			delete(js.repositories, repo.URL)
			return foundation.Err[*Repository, error](fmt.Errorf("failed to save repository: %s", err.Error()))
		}
	}
	return foundation.Ok[*Repository, error](repo)
}

// RepositoryGetByURL retrieves a repository by its URL.
func (js *JSONStore) RepositoryGetByURL(_ context.Context, url string) foundation.Result[*Repository, error] {
	if url == "" {
		return foundation.Err[*Repository, error](errors.New("URL cannot be empty"))
	}
	js.mu.RLock()
	defer js.mu.RUnlock()
	if repo, exists := js.repositories[url]; exists {
		repoCopy := *repo
		return foundation.Ok[*Repository, error](&repoCopy)
	}
	return foundation.Err[*Repository, error](derrors.NewError(derrors.CategoryNotFound, "repository not found: "+url).Build())
}

// RepositoryUpdate updates an existing repository.
func (js *JSONStore) RepositoryUpdate(_ context.Context, repo *Repository) foundation.Result[*Repository, error] {
	if repo == nil {
		return foundation.Err[*Repository, error](errors.New("repository cannot be nil"))
	}
	if validationResult := repo.Validate(); !validationResult.Valid {
		return foundation.Err[*Repository, error](validationResult.ToError())
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	if _, ok := js.repositories[repo.URL]; !ok {
		return foundation.Err[*Repository, error](fmt.Errorf("repository not found: %s", repo.URL))
	}
	repo.UpdatedAt = time.Now()
	js.repositories[repo.URL] = repo
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*Repository, error](fmt.Errorf("failed to save repository update: %s", err.Error()))
		}
	}
	return foundation.Ok[*Repository, error](repo)
}

// RepositoryList returns all repositories, sorted by name.
func (js *JSONStore) RepositoryList(_ context.Context) foundation.Result[[]Repository, error] {
	js.mu.RLock()
	defer js.mu.RUnlock()
	repositories := make([]Repository, 0, len(js.repositories))
	for _, repo := range js.repositories {
		repositories = append(repositories, *repo)
	}
	sort.Slice(repositories, func(i, j int) bool {
		return repositories[i].Name < repositories[j].Name
	})
	return foundation.Ok[[]Repository, error](repositories)
}

// RepositoryDelete removes a repository by URL.
func (js *JSONStore) RepositoryDelete(_ context.Context, url string) foundation.Result[void, error] {
	js.mu.Lock()
	defer js.mu.Unlock()
	if _, exists := js.repositories[url]; !exists {
		return foundation.Err[void, error](derrors.NewError(derrors.CategoryNotFound, "repository not found: "+url).Build())
	}
	delete(js.repositories, url)
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[void, error](fmt.Errorf("failed to save repository deletion: %s", err.Error()))
		}
	}
	return foundation.Ok[void, error](void{})
}

// RepositoryIncrementBuildCount increments build counters for a repository.
func (js *JSONStore) RepositoryIncrementBuildCount(_ context.Context, url string, success bool) foundation.Result[void, error] {
	js.mu.Lock()
	defer js.mu.Unlock()
	repo, exists := js.repositories[url]
	if !exists {
		return foundation.Err[void, error](derrors.NewError(derrors.CategoryNotFound, "repository not found: "+url).Build())
	}
	now := time.Now()
	repo.LastBuild = foundation.Some(now)
	repo.BuildCount++
	if !success {
		repo.ErrorCount++
	}
	repo.UpdatedAt = now
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[void, error](fmt.Errorf("failed to save build count update: %s", err.Error()))
		}
	}
	return foundation.Ok[void, error](void{})
}

// RepositorySetDocumentCount updates the document count for a repository.
func (js *JSONStore) RepositorySetDocumentCount(_ context.Context, url string, count int) foundation.Result[void, error] {
	if count < 0 {
		return foundation.Err[void, error](errors.New("document count cannot be negative"))
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	repo, exists := js.repositories[url]
	if !exists {
		return foundation.Err[void, error](derrors.NewError(derrors.CategoryNotFound, "repository not found: "+url).Build())
	}
	repo.DocumentCount = count
	repo.UpdatedAt = time.Now()
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[void, error](fmt.Errorf("failed to save document count update: %s", err.Error()))
		}
	}
	return foundation.Ok[void, error](void{})
}

// RepositorySetDocFilesHash updates the document files hash for a repository.
func (js *JSONStore) RepositorySetDocFilesHash(_ context.Context, url, hash string) foundation.Result[void, error] {
	js.mu.Lock()
	defer js.mu.Unlock()
	repo, exists := js.repositories[url]
	if !exists {
		return foundation.Err[void, error](derrors.NewError(derrors.CategoryNotFound, "repository not found: "+url).Build())
	}
	repo.DocFilesHash = foundation.Some(hash)
	repo.UpdatedAt = time.Now()
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[void, error](fmt.Errorf("failed to save doc files hash update: %s", err.Error()))
		}
	}
	return foundation.Ok[void, error](void{})
}

// RepositorySetDocFilePaths updates the document file paths for a repository.
func (js *JSONStore) RepositorySetDocFilePaths(_ context.Context, url string, paths []string) foundation.Result[void, error] {
	js.mu.Lock()
	defer js.mu.Unlock()
	repo, exists := js.repositories[url]
	if !exists {
		return foundation.Err[void, error](derrors.NewError(derrors.CategoryNotFound, "repository not found: "+url).Build())
	}
	repo.DocFilePaths = append([]string{}, paths...)
	repo.UpdatedAt = time.Now()
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[void, error](fmt.Errorf("failed to save doc file paths update: %s", err.Error()))
		}
	}
	return foundation.Ok[void, error](void{})
}

// --- Configuration operations ---

// ConfigurationSet stores a configuration value.
func (js *JSONStore) ConfigurationSet(_ context.Context, key string, value any) foundation.Result[void, error] {
	if key == "" {
		return foundation.Err[void, error](errors.New("configuration key cannot be empty"))
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	js.configuration[key] = value
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[void, error](fmt.Errorf("failed to save configuration: %s", err.Error()))
		}
	}
	return foundation.Ok[void, error](void{})
}

// ConfigurationGet retrieves a configuration value.
func (js *JSONStore) ConfigurationGet(_ context.Context, key string) foundation.Result[foundation.Option[any], error] {
	js.mu.RLock()
	defer js.mu.RUnlock()
	if v, ok := js.configuration[key]; ok {
		return foundation.Ok[foundation.Option[any], error](foundation.Some[any](v))
	}
	return foundation.Ok[foundation.Option[any], error](foundation.None[any]())
}

// ConfigurationDelete removes a configuration key.
func (js *JSONStore) ConfigurationDelete(_ context.Context, key string) foundation.Result[void, error] {
	js.mu.Lock()
	defer js.mu.Unlock()
	delete(js.configuration, key)
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[void, error](fmt.Errorf("failed to save configuration deletion: %s", err.Error()))
		}
	}
	return foundation.Ok[void, error](void{})
}

// ConfigurationList returns all configuration keys and values.
func (js *JSONStore) ConfigurationList(_ context.Context) foundation.Result[map[string]any, error] {
	js.mu.RLock()
	defer js.mu.RUnlock()
	cp := make(map[string]any, len(js.configuration))
	maps.Copy(cp, js.configuration)
	return foundation.Ok[map[string]any, error](cp)
}

// --- Daemon info operations ---

// DaemonInfoGet retrieves daemon information.
func (js *JSONStore) DaemonInfoGet(_ context.Context) foundation.Result[*DaemonInfo, error] {
	js.mu.RLock()
	defer js.mu.RUnlock()
	if js.daemonInfo == nil {
		return foundation.Err[*DaemonInfo, error](errors.New("daemon info not initialized"))
	}
	cp := *js.daemonInfo
	return foundation.Ok[*DaemonInfo, error](&cp)
}

// DaemonInfoUpdate updates daemon information.
func (js *JSONStore) DaemonInfoUpdate(_ context.Context, info *DaemonInfo) foundation.Result[*DaemonInfo, error] {
	if info == nil {
		return foundation.Err[*DaemonInfo, error](errors.New("daemon info cannot be nil"))
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	js.daemonInfo = info
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[*DaemonInfo, error](fmt.Errorf("failed to save daemon info update: %s", err.Error()))
		}
	}
	return foundation.Ok[*DaemonInfo, error](info)
}

// DaemonInfoUpdateStatus updates only the daemon status.
func (js *JSONStore) DaemonInfoUpdateStatus(_ context.Context, status string) foundation.Result[void, error] {
	if status == "" {
		return foundation.Err[void, error](errors.New("status cannot be empty"))
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	js.daemonInfo.Status = status
	if js.autoSaveEnabled {
		if err := js.saveToDiskUnsafe(); err != nil {
			return foundation.Err[void, error](fmt.Errorf("failed to save daemon status update: %s", err.Error()))
		}
	}
	return foundation.Ok[void, error](void{})
}
