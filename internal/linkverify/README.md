# Link Verification Package

Automated link validation for DocBuilder daemon mode with NATS event publishing.

## Overview

The `linkverify` package provides background link verification that runs after each successful build in daemon mode. It checks both internal and external links, caches results in NATS KV store, and publishes broken link events for downstream processing (e.g., automated forge issue creation).

## Architecture

### Components

1. **VerificationService** - Main orchestrator for link checking
2. **NATSClient** - NATS connection and KV cache management (link results + page hashes)
3. **Link Extractor** - HTML parsing to extract all links
4. **BrokenLinkEvent** - Rich event structure for NATS publishing

### Caching Strategy

The system uses a two-level caching approach in NATS KV:

**Bucket expiry:** The KV bucket is configured with a JetStream TTL so old keys expire automatically and storage shrinks over time. The bucket TTL is set to the larger of `cache_ttl` and `cache_ttl_failures`.

1. **Link-level cache**: Stores verification results per URL (MD5 hash of URL as key)
   - Successful checks cached for `cache_ttl` (default: 24h)
   - Failed checks cached for `cache_ttl_failures` (default: 1h)
   - Tracks failure counts and consecutive failures

2. **Page-level cache**: Stores MD5 hash of page HTML content (key: `page:{path_hash}`)
   - Skips link verification entirely if page content hasn't changed
   - Dramatically reduces verification load for unchanged pages
   - Cache keys prefixed with `page:` to distinguish from link cache

### Flow

```
Build Completes → Collect Pages → Hash Pages → Skip Unchanged → Extract Links → Verify (with cache) → Publish Events → Update Page Hashes
```

## Configuration

Add to your `config.yaml` under `daemon`:

```yaml
daemon:
  link_verification:
    enabled: true                      # Default: true
    nats_url: "nats://localhost:4222"  # NATS server
    subject: "docbuilder.links.broken" # Event subject
    kv_bucket: "docbuilder-link-cache" # Cache bucket
    cache_ttl: "24h"                   # Success cache TTL
    cache_ttl_failures: "1h"           # Failure cache TTL
    max_concurrent: 10                 # Parallel checks
    request_timeout: "10s"             # HTTP timeout
    rate_limit_delay: "100ms"          # Delay between requests
    verify_external_only: false        # Check both internal/external
    skip_edit_links: true              # Skip edit links (require auth, default: true)
    follow_redirects: true             # Follow HTTP redirects
    max_redirects: 3                   # Max redirect depth
```

## Event Structure

Published to NATS subject when broken links are found:

```json
{
  "url": "https://example.com/broken",
  "status": 404,
  "error": "HTTP 404: Not Found",
  "is_internal": false,
  
  "source_path": "/workspace/repo/docs/guide.md",
  "source_relative_path": "guide.md",
  "repository": "my-repo",
  "forge": "github-org",
  "section": "guides",
  "file_name": "guide",
  "docs_base": "docs",
  
  "title": "User Guide",
  "description": "Complete user guide",
  "front_matter": {...},
  
  "hugo_path": "content/my-repo/guides/guide.md",
  "rendered_path": "my-repo/guides/guide/index.html",
  "rendered_url": "https://docs.example.com/my-repo/guides/guide/",
  
  "timestamp": "2025-12-15T10:30:00Z",
  "last_checked": "2025-12-15T10:30:00Z",
  "failure_count": 3,
  "first_failed_at": "2025-12-14T10:00:00Z",
  
  "build_id": "build-1734264600",
  "build_time": "2025-12-15T10:30:00Z"
}
```

## Usage

### Automatic (Daemon Mode)

Link verification runs automatically after successful builds when enabled in daemon mode. No manual intervention required.

### Manual Verification

```go
import "git.home.luguber.info/inful/docbuilder/internal/linkverify"

// Create service
service, err := linkverify.NewVerificationService(cfg.Daemon.LinkVerification)
if err != nil {
    return err
}
defer service.Close()

// Collect pages
pages := []*linkverify.PageMetadata{...}

// Verify
err = service.VerifyPages(context.Background(), pages)
```

## Features

### NATS Connection Resilience
- **Automatic Reconnection**: Infinite reconnect attempts with exponential backoff
- **Non-Fatal Failures**: NATS unavailability doesn't crash the daemon
- **Graceful Degradation**: Continues verification without cache when NATS is down
- **Connection Handlers**: Logs reconnection events and reinitializes KV bucket
- **Lazy Connection**: Service can be created even when NATS is unavailable

### Link Extraction
- Extracts links from `<a>`, `<img>`, `<script>`, `<link>`, `<iframe>`, `<video>`, `<audio>`, `<source>` tags
- Classifies as internal or external based on site base URL
- Filters special protocols (mailto:, tel:, javascript:, data:)

### Verification
- **Internal links**: Checks file existence in rendered site
- **External links**: HEAD requests with configurable timeout
- **Rate limiting**: Configurable delay between checks
- **Concurrency**: Semaphore-based parallel checking

### Caching
- NATS KV store for distributed cache
- Separate TTLs for successes and failures
- Tracks consecutive failure counts
- Records first failure time for trending

### Events
- Comprehensive page metadata for issue automation
- Front matter extraction (title, description, custom fields)
- Build context (ID, timestamp)
- Verification history (failure count, first failure)

## Integration Points

### Daemon
Verification is triggered in `daemon.EmitBuildReport()` after successful builds:

```go
if report.Outcome == hugo.OutcomeSuccess && d.linkVerifier != nil {
    go d.verifyLinksAfterBuild(buildID, report)
}
```

### Issue Automation
Downstream services subscribe to `docbuilder.links.broken` and create forge issues:

```go
// External service (not in this repo)
sub, _ := nc.Subscribe("docbuilder.links.broken", func(m *nats.Msg) {
    var event linkverify.BrokenLinkEvent
    json.Unmarshal(m.Data, &event)
    
    // Create issue in forge
    createIssue(event.Repository, event.Forge, event)
})
```

## Performance

- **Low priority**: Runs in background goroutine
- **Non-blocking**: Doesn't impact build pipeline
- **Cached**: Avoids redundant checks
- **Rate-limited**: Respectful of external sites
- **Concurrent**: Configurable parallelism

## Best Practices

1. **Set appropriate TTLs** based on your content update frequency
2. **Adjust rate limits** to avoid overwhelming external sites
3. **Monitor NATS KV size** - it should shrink over time due to bucket TTL; consider raising limits if you still hit max bytes
4. **Subscribe to events** for automated issue creation
5. **Track failure counts** to identify persistent issues

## Error Handling

- **NATS Connection Failures**: Service creation succeeds even if NATS is down
  - Initial connection attempt logs warning
  - Automatic reconnection on first operation
  - Infinite reconnect attempts with 2s base interval + jitter
  - Operations gracefully degrade (skip cache) when NATS unavailable
- **Event Publishing Failures**: Logged as errors but don't fail verification
- **Cache Failures**: Logged but verification continues without cache
- **Individual Link Failures**: Published as events, don't block other links
- **Service Shutdown**: Gracefully handles context cancellation
- **Close Errors**: Logged but not propagated

## Testing

```bash
# Unit tests
go test ./internal/linkverify -v

# Integration test with NATS
docker run -d -p 4222:4222 nats:latest
go test ./internal/daemon -v -run TestLinkVerification
```

## Dependencies

- `github.com/nats-io/nats.go` - NATS client
- `github.com/nats-io/nats.go/jetstream` - JetStream (KV)
- `golang.org/x/net/html` - HTML parsing
- `gopkg.in/yaml.v3` - Front matter parsing

## Future Enhancements

- [ ] Link validation rules (skip patterns, whitelists)
- [ ] Automatic retry on transient failures
- [ ] Metrics and Prometheus integration
- [ ] Link health dashboard
- [ ] Batch verification API
- [ ] Custom notification targets (Slack, webhooks)
