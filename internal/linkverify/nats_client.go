package linkverify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSClient manages NATS connection and operations for link verification.
type NATSClient struct {
	conn     *nats.Conn
	js       jetstream.JetStream
	kv       jetstream.KeyValue
	cfg      *config.LinkVerificationConfig
	subject  string
	kvBucket string
}

// NewNATSClient creates a new NATS client for link verification.
func NewNATSClient(cfg *config.LinkVerificationConfig) (*NATSClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("link verification config is required")
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("link verification is disabled")
	}

	// Connect to NATS
	conn, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	client := &NATSClient{
		conn:     conn,
		js:       js,
		cfg:      cfg,
		subject:  cfg.Subject,
		kvBucket: cfg.KVBucket,
	}

	// Initialize KV bucket
	if err := client.initKVBucket(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize KV bucket: %w", err)
	}

	slog.Info("NATS client initialized for link verification",
		"url", cfg.NATSURL,
		"subject", cfg.Subject,
		"kv_bucket", cfg.KVBucket)

	return client, nil
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	event.Timestamp = time.Now()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = c.js.Publish(ctx, c.subject, data)
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	entry, err := c.kv.Get(ctx, url)
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	entry.LastChecked = time.Now()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	// Note: NATS KV doesn't support per-key TTL in current API
	// TTL is checked when retrieving entries via IsCacheValid()

	// Put entry in KV store
	_, err = c.kv.Put(ctx, entry.URL, data)
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
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}
