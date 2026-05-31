package discoveryrunner

import (
	"sync"

	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// Cache caches the most recent repository discovery result.
// This enables fast responses to status endpoint queries without
// repeating expensive network operations.
type Cache struct {
	mu     sync.RWMutex
	result *forge.DiscoveryResult
	err    error
}

// NewCache creates a new Cache.
func NewCache() *Cache {
	return &Cache{}
}

// Update stores the latest discovery result and clears any previous error.
func (c *Cache) Update(result *forge.DiscoveryResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.result = result
	c.err = nil
}

// Merge performs an atomic read-modify-write of the cached result.
// If mergeFn or its return value is nil, the cache is left unchanged.
// Successful merges clear any previous cached error.
func (c *Cache) Merge(mergeFn func(current *forge.DiscoveryResult) *forge.DiscoveryResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if mergeFn == nil {
		return
	}

	next := mergeFn(c.result)
	if next == nil {
		return
	}

	c.result = next
	c.err = nil
}

// SetError stores a discovery error, preserving the previous result (if any).
func (c *Cache) SetError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = err
}

// Get returns the cached discovery result and any error.
// Returns (nil, nil) if no discovery has been performed yet.
func (c *Cache) Get() (*forge.DiscoveryResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.result, c.err
}

// GetResult returns just the cached discovery result (may be nil).
func (c *Cache) GetResult() *forge.DiscoveryResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.result
}

// GetError returns just the cached error (may be nil).
func (c *Cache) GetError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.err
}

// HasResult returns true if a discovery result is cached.
func (c *Cache) HasResult() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.result != nil
}

// Clear removes the cached result and error.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.result = nil
	c.err = nil
}
