package linkverify

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"git.home.luguber.info/inful/docbuilder/internal/config"
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
		return nil, errors.New("link verification config is required")
	}

	if !cfg.Enabled {
		return nil, errors.New("link verification is disabled")
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
	return c.connectWithContext(context.Background())
}

func (c *NATSClient) connectWithContext(ctx context.Context) error {
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
		//nolint:contextcheck // ReconnectHandler is a callback without parent context
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected", "url", nc.ConnectedUrl())
			c.reconnecting.Store(false)
			// Reinitialize KV bucket after reconnect
			if err := c.initKVBucket(context.Background()); err != nil {
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
	if err := c.initKVBucket(ctx); err != nil {
		conn.Close()
		c.conn = nil
		c.js = nil
		return fmt.Errorf("failed to initialize KV bucket: %w", err)
	}

	// Initialize stream for broken link events
	if err := c.initStream(ctx); err != nil {
		// Non-fatal: stream is optional for caching to work
		slog.Warn("Failed to initialize NATS stream for broken link events",
			"error", err,
			"subject", c.subject)
	}

	slog.Info("NATS client connected",
		"url", c.cfg.NATSURL,
		"subject", c.subject,
		"kv_bucket", c.kvBucket)

	return nil
}

// ensureConnected checks connection and reconnects if necessary.
func (c *NATSClient) ensureConnected(ctx context.Context) error {
	c.mu.RLock()
	connected := c.conn != nil && c.conn.IsConnected()
	c.mu.RUnlock()

	if connected {
		return nil
	}

	// Avoid multiple concurrent reconnection attempts
	if c.reconnecting.Swap(true) {
		return errors.New("reconnection already in progress")
	}
	defer c.reconnecting.Store(false)

	slog.Info("Attempting to reconnect to NATS", "url", c.cfg.NATSURL)
	return c.connectWithContext(ctx)
}

// initKVBucket creates or gets the KV bucket for link cache.
func (c *NATSClient) initKVBucket(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Try to get existing bucket
	kv, err := c.js.KeyValue(timeoutCtx, c.kvBucket)
	if err == nil {
		c.kv = kv
		return nil
	}

	// Create new bucket if it doesn't exist
	kv, err = c.js.CreateKeyValue(timeoutCtx, jetstream.KeyValueConfig{
		Bucket:      c.kvBucket,
		Description: "Link verification cache for DocBuilder",
		MaxBytes:    100 * 1024 * 1024, // 100MB max
		History:     1,                 // Keep only latest value
		TTL:         0,                 // Per-key TTL
	})
	if err != nil {
		return fmt.Errorf("failed to create KV bucket: %w", err)
	}

	c.kv = kv
	slog.Info("Created KV bucket for link cache", "bucket", c.kvBucket)
	return nil
}

// initStream creates or gets the JetStream stream for broken link events.
func (c *NATSClient) initStream(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	streamName := "DOCBUILDER_LINKS"

	// Try to get existing stream
	_, err := c.js.Stream(timeoutCtx, streamName)
	if err == nil {
		return nil // Stream already exists
	}

	// Create new stream if it doesn't exist
	_, err = c.js.CreateStream(timeoutCtx, jetstream.StreamConfig{
		Name:        streamName,
		Description: "Broken link events from DocBuilder link verification",
		Subjects:    []string{c.subject},
		Retention:   jetstream.LimitsPolicy,
		MaxMsgs:     10000,              // Keep last 10k broken link events
		MaxBytes:    50 * 1024 * 1024,   // 50MB max
		MaxAge:      7 * 24 * time.Hour, // Keep events for 7 days
		Storage:     jetstream.FileStorage,
		Discard:     jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	slog.Info("Created NATS stream for broken link events",
		"stream", streamName,
		"subject", c.subject)
	return nil
}

// PublishBrokenLink publishes a broken link event to NATS.
func (c *NATSClient) PublishBrokenLink(ctx context.Context, event *BrokenLinkEvent) error {
	// Ensure we're connected before publishing
	if err := c.ensureConnected(ctx); err != nil {
		return fmt.Errorf("NATS not connected: %w", err)
	}

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
		return errors.New("JetStream not available")
	}

	_, err = js.Publish(pubCtx, c.subject, data)
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
	URL             string    `json:"url"`
	Status          int       `json:"status"`
	IsValid         bool      `json:"is_valid"`
	Error           string    `json:"error,omitempty"`
	LastChecked     time.Time `json:"last_checked"`
	FailureCount    int       `json:"failure_count"`
	FirstFailedAt   time.Time `json:"first_failed_at,omitzero"`
	ConsecutiveFail bool      `json:"consecutive_fail"`
}

// GetCachedResult retrieves a cached link verification result.
func (c *NATSClient) GetCachedResult(ctx context.Context, url string) (*CacheEntry, error) {
	// Ensure we're connected before accessing cache
	if err := c.ensureConnected(ctx); err != nil {
		slog.Debug("NATS not connected, skipping cache lookup", "error", err)
		return nil, nil // Return nil to indicate cache miss
	}

	cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	c.mu.RLock()
	kv := c.kv
	c.mu.RUnlock()

	if kv == nil {
		return nil, nil // Cache not available
	}

	// Use MD5 hash as KV key
	key := sanitizeKVKey(url)

	entry, err := kv.Get(cacheCtx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
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
func (c *NATSClient) SetCachedResult(ctx context.Context, entry *CacheEntry) error {
	// Ensure we're connected before updating cache
	if err := c.ensureConnected(ctx); err != nil {
		slog.Debug("NATS not connected, skipping cache update", "error", err)
		return nil // Non-fatal - continue without caching
	}

	cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
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

	// Sanitize URL for use as KV key (NATS keys can't contain /, ?, #, etc.)
	key := sanitizeKVKey(entry.URL)

	// Put entry in KV store
	_, err = kv.Put(cacheCtx, key, data)
	if err != nil {
		return fmt.Errorf("failed to put cache entry: %w", err)
	}

	return nil
}

// sanitizeKVKey converts a URL into a valid NATS KV key using MD5 hash.
// NATS KV keys must match [a-zA-Z0-9_-]+ (no slashes, query params, etc.)
func sanitizeKVKey(url string) string {
	hash := md5.Sum([]byte(url))
	return hex.EncodeToString(hash[:])
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

// GetPageHash retrieves the cached MD5 hash for a page path.
func (c *NATSClient) GetPageHash(ctx context.Context, pagePath string) (string, error) {
	// Ensure we're connected
	if err := c.ensureConnected(ctx); err != nil {
		return "", err
	}

	hashCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	c.mu.RLock()
	kv := c.kv
	c.mu.RUnlock()

	if kv == nil {
		return "", errors.New("KV bucket not available")
	}

	// Use page_ prefix to distinguish from link cache entries
	// NATS KV keys must match ^[a-zA-Z0-9_-]+$ (no colons allowed)
	key := "page_" + sanitizeKVKey(pagePath)

	entry, err := kv.Get(hashCtx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return "", errors.New("page hash not cached")
		}
		return "", fmt.Errorf("failed to get page hash: %w", err)
	}

	return string(entry.Value()), nil
}

// SetPageHash stores the MD5 hash for a page path in the cache.
func (c *NATSClient) SetPageHash(ctx context.Context, pagePath, hash string) error {
	// Ensure we're connected
	if err := c.ensureConnected(ctx); err != nil {
		slog.Debug("NATS not connected, skipping page hash update", "error", err)
		return nil // Non-fatal
	}

	hashCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	c.mu.RLock()
	kv := c.kv
	c.mu.RUnlock()

	if kv == nil {
		return nil // Cache not available - non-fatal
	}

	// Use page_ prefix to distinguish from link cache entries
	// NATS KV keys must match ^[a-zA-Z0-9_-]+$ (no colons allowed)
	key := "page_" + sanitizeKVKey(pagePath)

	_, err := kv.Put(hashCtx, key, []byte(hash))
	if err != nil {
		return fmt.Errorf("failed to put page hash: %w", err)
	}

	return nil
}
