package linkverify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSClient manages NATS connection and operations for link verification.
type NATSClient struct {
	conn         *nats.Conn
	js           jetstream.JetStream
	kv           jetstream.KeyValue
	cfg          *config.LinkVerificationConfig
	subject      string
	kvBucket     string
	mu           sync.RWMutex
	reconnecting atomic.Bool
}

// NewNATSClient creates a new NATS client for link verification.
// Connection failures are non-fatal; the client will attempt to reconnect automatically.
func NewNATSClient(cfg *config.LinkVerificationConfig) (*NATSClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("link verification config is required")
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("link verification is disabled")
	}

	client := &NATSClient{
		cfg:      cfg,
		subject:  cfg.Subject,
		kvBucket: cfg.KVBucket,
	}

	// Attempt initial connection (non-fatal if fails)
	if err := client.connect(); err != nil {
		slog.Warn("Initial NATS connection failed, will retry on first use",
			"url", cfg.NATSURL,
			"error", err)
		// Return client anyway - it will reconnect on first use
	}

	return client, nil
}

// connect establishes connection to NATS server with reconnect handlers.
func (c *NATSClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing connection if any
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.js = nil
		c.kv = nil
	}

	// Configure connection options with automatic reconnection
	opts := []nats.Option{
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.ReconnectWait(2 * time.Second),
		nats.ReconnectJitter(500*time.Millisecond, 2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				slog.Warn("NATS disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected", "url", nc.ConnectedUrl())
			c.reconnecting.Store(false)
			// Reinitialize KV bucket after reconnect
			if err := c.initKVBucket(); err != nil {
				slog.Error("Failed to reinitialize KV bucket after reconnect", "error", err)
			}
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			slog.Info("NATS connection closed")
		}),
	}

	// Connect to NATS
	conn, err := nats.Connect(c.cfg.NATSURL, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create JetStream context: %w", err)
	}

	c.conn = conn
	c.js = js

	// Initialize KV bucket
	if err := c.initKVBucket(); err != nil {
		conn.Close()
		c.conn = nil
		c.js = nil
		return fmt.Errorf("failed to initialize KV bucket: %w", err)
	}

	slog.Info("NATS client connected",
		"url", c.cfg.NATSURL,
		"subject", c.subject,
		"kv_bucket", c.kvBucket)

	return nil
}

// ensureConnected checks connection and reconnects if necessary.
func (c *NATSClient) ensureConnected() error {
	c.mu.RLock()
	connected := c.conn != nil && c.conn.IsConnected()
	c.mu.RUnlock()

	if connected {
		return nil
	}

	// Avoid multiple concurrent reconnection attempts
	if c.reconnecting.Swap(true) {
		return fmt.Errorf("reconnection already in progress")
	}
	defer c.reconnecting.Store(false)

	slog.Info("Attempting to reconnect to NATS", "url", c.cfg.NATSURL)
	return c.connect()
}

// initKVBucket creates or gets the KV bucket for link cache.
func (c *NATSClient) initKVBucket() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to get existing bucket
	kv, err := c.js.KeyValue(ctx, c.kvBucket)
	if err == nil {
		c.kv = kv
		return nil
	}

	// Create new bucket if it doesn't exist
	kv, err = c.js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      c.kvBucket,
		Description: "Link verification cache for DocBuilder",
		MaxBytes:    100 * 1024 * 1024, // 100MB max
		History:     1,                   // Keep only latest value
		TTL:         0,                   // Per-key TTL
	})
	if err != nil {
		return fmt.Errorf("failed to create KV bucket: %w", err)
	}

	c.kv = kv
	slog.Info("Created KV bucket for link cache", "bucket", c.kvBucket)
	return nil
}

// PublishBrokenLink publishes a broken link event to NATS.
func (c *NATSClient) PublishBrokenLink(event *BrokenLinkEvent) error {
	// Ensure we're connected before publishing
	if err := c.ensureConnected(); err != nil {
		return fmt.Errorf("NATS not connected: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	event.Timestamp = time.Now()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	c.mu.RLock()
	js := c.js
	c.mu.RUnlock()

	if js == nil {
		return fmt.Errorf("JetStream not available")
	}

	_, err = js.Publish(ctx, c.subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	slog.Debug("Published broken link event",
		"url", event.URL,
		"source", event.SourceRelativePath,
		"repository", event.Repository)

	return nil
}

// CacheEntry represents a cached link verification result.
type CacheEntry struct {
	URL            string    `json:"url"`
	Status         int       `json:"status"`
	IsValid        bool      `json:"is_valid"`
	Error          string    `json:"error,omitempty"`
	LastChecked    time.Time `json:"last_checked"`
	FailureCount   int       `json:"failure_count"`
	FirstFailedAt  time.Time `json:"first_failed_at,omitempty"`
	ConsecutiveFail bool     `json:"consecutive_fail"`
}

// GetCachedResult retrieves a cached link verification result.
func (c *NATSClient) GetCachedResult(url string) (*CacheEntry, error) {
	// Ensure we're connected before accessing cache
	if err := c.ensureConnected(); err != nil {
		slog.Debug("NATS not connected, skipping cache lookup", "error", err)
		return nil, nil // Return nil to indicate cache miss
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c.mu.RLock()
	kv := c.kv
	c.mu.RUnlock()

	if kv == nil {
		return nil, nil // Cache not available
	}

	entry, err := kv.Get(ctx, url)
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			return nil, nil // Not cached
		}
		return nil, fmt.Errorf("failed to get cache entry: %w", err)
	}

	var cached CacheEntry
	if err := json.Unmarshal(entry.Value(), &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache entry: %w", err)
	}

	return &cached, nil
}

// SetCachedResult stores a link verification result in cache.
func (c *NATSClient) SetCachedResult(entry *CacheEntry) error {
	// Ensure we're connected before updating cache
	if err := c.ensureConnected(); err != nil {
		slog.Debug("NATS not connected, skipping cache update", "error", err)
		return nil // Non-fatal - continue without caching
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	entry.LastChecked = time.Now()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	// Note: NATS KV doesn't support per-key TTL in current API
	// TTL is checked when retrieving entries via IsCacheValid()

	c.mu.RLock()
	kv := c.kv
	c.mu.RUnlock()

	if kv == nil {
		return nil // Cache not available - non-fatal
	}

	// Put entry in KV store
	_, err = kv.Put(ctx, entry.URL, data)
	if err != nil {
		return fmt.Errorf("failed to put cache entry: %w", err)
	}

	return nil
}

// IsCacheValid checks if a cache entry is still valid based on TTL.
func (c *NATSClient) IsCacheValid(entry *CacheEntry) bool {
	if entry == nil {
		return false
	}

	var ttl time.Duration
	if entry.IsValid {
		ttl, _ = time.ParseDuration(c.cfg.CacheTTL)
	} else {
		ttl, _ = time.ParseDuration(c.cfg.CacheTTLFailures)
	}

	return time.Since(entry.LastChecked) < ttl
}

// Close closes the NATS connection.
func (c *NATSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.js = nil
		c.kv = nil
	}
	return nil
}
