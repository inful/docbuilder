package daemon

import (
	"sync"

	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// DiscoveryCache caches the most recent repository discovery result.
// This enables fast responses to status endpoint queries without
// repeating expensive network operations.
type DiscoveryCache struct {
	mu     sync.RWMutex
	result *forge.DiscoveryResult
	err    error
}

// NewDiscoveryCache creates a new DiscoveryCache.
func NewDiscoveryCache() *DiscoveryCache {
	return &DiscoveryCache{}
}

// Update stores the latest discovery result and clears any previous error.
func (c *DiscoveryCache) Update(result *forge.DiscoveryResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.result = result
	c.err = nil
}

// SetError stores a discovery error, preserving the previous result (if any).
func (c *DiscoveryCache) SetError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = err
}

// Get returns the cached discovery result and any error.
// Returns (nil, nil) if no discovery has been performed yet.
func (c *DiscoveryCache) Get() (*forge.DiscoveryResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.result, c.err
}

// GetResult returns just the cached discovery result (may be nil).
func (c *DiscoveryCache) GetResult() *forge.DiscoveryResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.result
}

// GetError returns just the cached error (may be nil).
func (c *DiscoveryCache) GetError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.err
}

// HasResult returns true if a discovery result is cached.
func (c *DiscoveryCache) HasResult() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.result != nil
}

// Clear removes the cached result and error.
func (c *DiscoveryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.result = nil
	c.err = nil
}
