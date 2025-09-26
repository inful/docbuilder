# ADR-001: Forge Integration and Daemon Mode

**Status**: Proposed  
**Date**: 2025-09-26  
**Decision Makers**: Architecture Team  
**Technical Story**: Extend DocBuilder with automatic forge discovery and daemon mode

## Context and Problem Statement

The current DocBuilder system requires manual configuration of each repository and operates as a one-time CLI tool. Organizations with many repositories find it cumbersome to:

1. Manually maintain lists of repositories with documentation
2. Remember to update documentation builds when repositories change
3. Deploy and serve the generated documentation
4. Keep documentation current across dozens or hundreds of repositories

We need to evolve DocBuilder into an automated documentation aggregation platform that can:
- Automatically discover repositories with documentation across multiple forge platforms
- Continuously synchronize with repository changes
- Serve the generated documentation with operational monitoring

## Decision

We will extend DocBuilder with **Forge Integration** and **Daemon Mode** capabilities, transforming it from a CLI tool into a comprehensive documentation platform.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    DocBuilder Daemon                        │
├─────────────────────────────────────────────────────────────┤
│  HTTP Server                                                │
│  ├── Documentation Serving (Hugo static files)             │
│  ├── Webhook Endpoints (/webhooks/{forge})                 │
│  ├── Status/Admin API (/status, /health, /metrics)         │
│  └── Configuration Reload (/admin/reload)                  │
├─────────────────────────────────────────────────────────────┤
│  Forge Integration Layer                                    │
│  ├── GitHub API Client                                     │
│  ├── GitLab API Client                                     │
│  ├── Forgejo API Client                                    │
│  └── Generic Forge Interface                               │
├─────────────────────────────────────────────────────────────┤
│  Repository Management                                      │
│  ├── Discovery Service (periodic org/group scanning)      │
│  ├── Sync Queue (deduplicating rebuild requests)          │
│  ├── Git Operations (clone, pull, branch management)      │
│  └── State Management (file-based persistent storage)     │
├─────────────────────────────────────────────────────────────┤
│  Documentation Pipeline                                    │
│  ├── Documentation Discovery (existing logic)             │
│  ├── Multi-version/branch Support                         │
│  ├── Hugo Site Generation (enhanced)                      │
│  └── Status Page Generation (integrated)                  │
└─────────────────────────────────────────────────────────────┘
```

## Detailed Requirements

### 1. Forge Integration

**Multi-Forge Support**: Support GitHub, GitLab, and Forgejo simultaneously
- Abstract forge interface with platform-specific implementations
- Per-forge authentication (PATs, OAuth, etc.)
- Organization/group level repository discovery
- Respect repository permissions (only include accessible repos)

**Repository Filtering**:
- Include only repositories with `docs/` folder
- Exclude repositories with `.docignore` file in root
- Support additional filtering rules (naming patterns, topics, etc.)

**Lifecycle Management**:
- Automatically detect new repositories in configured organizations
- Remove documentation when repositories are deleted
- Handle repository renames/moves gracefully
- Track repository state changes

### 2. Daemon Mode

**HTTP Server Capabilities**:
- Serve Hugo-generated static documentation
- Receive and validate webhooks from multiple forges
- Provide status/health endpoints for monitoring
- Support configuration reload without restart

**Webhook Processing**:
- Signature validation per forge requirements
- Handle push events, repository events, and lifecycle changes
- Queue-based processing with deduplication per repository
- Non-blocking webhook processing (documentation rebuilds don't block webhook reception)

**Synchronization Strategy** (Hybrid Approach):
- **Scheduled Discovery**: Periodic scanning for new repositories in configured organizations
- **Webhook Updates**: Real-time updates for repository changes
- **Graceful Degradation**: Scheduled sync catches missed webhook events

### 3. State Management

**File-Based Persistent State**:
- Repository metadata (last sync, commit hashes, branches)
- Forge connection state and discovered repository lists
- Build history and status tracking
- Webhook registration state

**Status Integration**:
- Generate status page as part of Hugo documentation
- Show sync status, last update times, and error states
- Provide operational visibility into daemon health

### 4. Multi-Version Support

**Branch/Version Strategies**:
- Default branch documentation (always)
- Configurable additional branch patterns
- Tag-based versioning (e.g., v*.* tags)
- Per-repository version configuration

**Version Management**:
- Organize multi-version content in Hugo structure
- Version switcher in documentation interface
- Cleanup of old versions based on retention policies

### 5. Operational Features

**Service Management**:
- Graceful shutdown (complete current builds before stopping)
- Configuration hot-reload without service restart
- Health check endpoints for load balancer integration

**Observability**:
- Structured logging (JSON format for log aggregation)
- Prometheus-style metrics endpoint
- Built-in alerting for sync failures and operational issues
- Integration points for external monitoring systems

**Deployment**:
- Single binary with embedded Hugo (preferred)
- Fallback: Docker container with Hugo included
- Stateful deployment considerations for repository storage

## Configuration Design (v2 Format)

```yaml
# DocBuilder Daemon Configuration v2
version: "2.0"

daemon:
  http:
    docs_port: 8080      # Documentation serving
    webhook_port: 8081   # Webhook reception (can be same as docs_port)
    admin_port: 8082     # Admin/status endpoints
    
  sync:
    schedule: "0 */4 * * *"  # Cron expression for discovery
    concurrent_builds: 3      # Max parallel repository builds
    queue_size: 100          # Max queued build requests
    
  storage:
    state_file: "./docbuilder-state.json"
    repo_cache_dir: "./repositories"
    output_dir: "./site"

forges:
  - name: "company-github"
    type: "github"
    api_url: "https://api.github.com"
    organizations: ["mycompany", "mycompany-oss"]
    auth:
      type: "token"
      token: "${GITHUB_TOKEN}"
    webhook:
      secret: "${GITHUB_WEBHOOK_SECRET}"
      path: "/webhooks/github"
      
  - name: "internal-gitlab"
    type: "gitlab"
    api_url: "https://gitlab.company.com/api/v4"
    groups: ["engineering", "platform"]
    auth:
      type: "token"
      token: "${GITLAB_TOKEN}"
    webhook:
      secret: "${GITLAB_WEBHOOK_SECRET}"
      path: "/webhooks/gitlab"

filtering:
  required_paths: ["docs"]
  ignore_files: [".docignore"]
  include_patterns: ["*-docs", "documentation-*"]
  exclude_patterns: ["*-archive", "deprecated-*"]

versioning:
  strategy: "branches_and_tags"
  default_branch_only: false
  branch_patterns: ["main", "master", "develop", "release/*"]
  tag_patterns: ["v*.*.*", "release-*"]
  max_versions_per_repo: 10

hugo:
  title: "Company Documentation Portal"
  description: "Aggregated documentation from all engineering projects"
  base_url: "https://docs.company.com"
  theme: "hextra"
  
monitoring:
  metrics:
    enabled: true
    path: "/metrics"
  health:
    path: "/health"
  logging:
    level: "info"
    format: "json"
```

## Implementation Strategy

### Phase 1: Forge Integration Foundation ✅ **COMPLETED**
- ✅ Implement forge abstraction layer and API clients
- ✅ Add organization/group discovery capabilities  
- ✅ Extend repository filtering with `.docignore` support
- ✅ Create v2 configuration format and parsing
- ✅ Build organization discovery service with filtering

**Implementation Files Created:**
- `internal/forge/types.go` - Core interfaces and data structures
- `internal/forge/github.go` - GitHub API client with webhook support
- `internal/forge/gitlab.go` - GitLab API client for self-hosted and cloud
- `internal/forge/forgejo.go` - Forgejo/Gitea API client
- `internal/forge/factory.go` - Client factory and forge manager
- `internal/forge/discovery.go` - Organization discovery service
- `internal/config/v2.go` - V2 configuration format for daemon mode
- Enhanced `internal/docs/discovery.go` with `.docignore` filtering
- Enhanced `internal/git/git.go` with docignore checking

### Phase 2: Daemon Infrastructure  
- Implement daemon mode with HTTP server
- Implement webhook reception and validation
- Add queue-based build system with deduplication
- Create file-based state management

### Phase 3: Multi-Version and Advanced Features
- Add multi-branch/tag documentation support
- Implement status page generation and integration
- Add configuration hot-reload capabilities
- Enhance observability (metrics, health checks)

### Phase 4: Production Readiness
- Implement graceful shutdown and error recovery
- Add comprehensive monitoring and alerting
- Create deployment packaging (binary + Docker)
- Performance optimization and testing

## Consequences

### Positive
- **Automated Documentation**: Organizations can maintain up-to-date documentation with minimal manual intervention
- **Multi-Forge Support**: Works across different development platforms and hosting scenarios
- **Scalable Architecture**: Queue-based processing handles high repository counts and frequent updates
- **Operational Excellence**: Built-in monitoring and observability for production deployment
- **Developer Experience**: Status page provides transparency into documentation build process

### Negative
- **Complexity Increase**: Significantly more complex than the current CLI tool
- **Resource Requirements**: Daemon mode requires persistent infrastructure and state storage
- **Security Surface**: HTTP server and webhook processing introduce additional attack vectors
- **Deployment Complexity**: More configuration and operational considerations for deployment

### Risks and Mitigations

| Risk | Impact | Probability | Mitigation |
|------|---------|-------------|------------|
| API Rate Limiting | Service degradation | Medium | Implement backoff strategies and caching |
| Webhook Delivery Failures | Stale documentation | Medium | Scheduled sync as fallback mechanism |
| Repository Scale Issues | Performance problems | Low | Queue limits and concurrent processing controls |
| State File Corruption | Service failure | Low | State file backups and recovery procedures |

## Alternatives Considered

1. **External Queue System**: Using Redis/RabbitMQ instead of in-memory queue
   - **Rejected**: Adds deployment complexity for marginal benefit
   
2. **Database State Storage**: PostgreSQL/SQLite instead of file-based state
   - **Rejected**: File-based state is simpler and meets requirements
   
3. **Separate Services**: Split webhook handling, discovery, and serving into microservices
   - **Rejected**: Single binary deployment is preferred for simplicity

4. **Existing Tools Extension**: Extend tools like GitBook or Confluence
   - **Rejected**: Doesn't provide the multi-forge Git-native approach desired

## Related Decisions

- The existing CLI mode will remain available for backward compatibility
- Hugo remains the static site generator (no change)
- Git operations will be enhanced but maintain the same authentication patterns
- The current configuration format (v1) will be deprecated but supported

## References

- [Current DocBuilder Implementation](./feature-docbuilder-core-1.md)
- [GitHub API Documentation](https://docs.github.com/en/rest)
- [GitLab API Documentation](https://docs.gitlab.com/ee/api/)
- [Forgejo API Documentation](https://forgejo.org/docs/latest/user/api-usage/)
- [Hugo Documentation](https://gohugo.io/documentation/)