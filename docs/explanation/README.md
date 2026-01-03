# Architecture Documentation

Comprehensive architecture documentation for DocBuilder organized by topic and detail level.

## Core Architecture Documents

### [Architecture Overview](architecture-overview.md)
High-level system architecture covering:
- System layers and components
- Build pipeline stages
- Content transform pipeline
- Key design patterns
- Configuration management
- Observability

**Audience:** New developers, technical leads, product managers

### [Pipeline Architecture](pipeline-architecture.md)
Detailed pipeline stage documentation:
- Eight sequential build stages
- Stage operations and responsibilities
- Error handling and retries
- Incremental build mechanics
- Stage metrics and observability

**Audience:** Contributors, pipeline developers

### [Package Dependencies](package-dependencies.md)
Package structure and relationships:
- Package organization by layer
- Dependency rules and direction
- Key interfaces and exports
- Testing strategy

**Audience:** Contributors, maintainers

### [Data Flow](data-flow.md)
Data flow through the system:
- Configuration to rendered site
- Build request processing
- Content transformation steps
- Incremental build flow
- Event and metrics flow

**Audience:** Contributors, architects

### [Comprehensive Architecture](comprehensive-architecture.md)
Complete system design:
- Core principles (clean architecture, event sourcing, typed state)
- All subsystems and components
- Extension points
- Operational considerations

**Audience:** Senior engineers, architects

### [Package Architecture](package-architecture.md)
Detailed package documentation:
- Package-by-package descriptions
- Key types and interfaces
- Usage patterns and examples
- Design rationale

**Audience:** Contributors, package users

## Specialized Topics

### [Namespacing Rationale](namespacing-rationale.md)
Forge-level namespacing design for multi-forge setups.

### [Renderer Testing](renderer-testing.md)
Hugo rendering test strategies and golden test patterns.

### [Skip Evaluation](skip-evaluation.md)
Incremental build and change detection logic.

### [Webhook Documentation Isolation](webhook-documentation-isolation.md)
Webhook handling and event processing.

## Architecture Decision Records

Significant architectural decisions:
- [ADR-000: Uniform Error Handling](../adr/adr-000-uniform-error-handling.md)
- [ADR-001: Golden Testing Strategy](../adr/adr-001-golden-testing-strategy.md)
- [ADR-002: In-Memory Content Pipeline](../adr/adr-002-in-memory-content-pipeline.md)
- [ADR-003: Fixed Transform Pipeline](../adr/adr-003-fixed-transform-pipeline.md)
- [ADR-004: Forge-Specific Markdown](../adr/adr-004-forge-specific-markdown.md)
- [ADR-005: Documentation Linting](../adr/adr-005-documentation-linting.md)
- [ADR-006: Drop Local Namespace](../adr/adr-006-drop-local-namespace.md)

## Quick Navigation

### New Developers
1. [Architecture Overview](architecture-overview.md)
2. [Pipeline Architecture](pipeline-architecture.md)
3. [Package Dependencies](package-dependencies.md)

### Contributing to Pipeline
1. [Pipeline Architecture](pipeline-architecture.md)
2. [Data Flow](data-flow.md)
3. [Comprehensive Architecture](comprehensive-architecture.md)

### Understanding State Management
1. [Data Flow](data-flow.md)
2. [Comprehensive Architecture](comprehensive-architecture.md)
3. [Package Architecture](package-architecture.md)

## System Layers

```
┌─────────────────────────┐
│   Command Layer         │  cmd/docbuilder/commands
├─────────────────────────┤
│   Service Layer         │  internal/build, internal/daemon
├─────────────────────────┤
│   Processing Layer      │  internal/hugo, internal/docs
├─────────────────────────┤
│   Domain Layer          │  internal/config, internal/state
├─────────────────────────┤
│   Infrastructure Layer  │  internal/git, internal/forge
├─────────────────────────┤
│   Foundation Layer      │  internal/foundation
└─────────────────────────┘
```

## Key Principles

1. **Clean Architecture** - Clear dependency direction (inward only)
2. **Event Sourcing** - Immutable event log for build lifecycle
3. **Typed State** - Strong typing, no `map[string]any` in core paths
4. **Unified Errors** - Classified errors with categories
5. **Observable** - Structured logging, Prometheus metrics, build reports

## Related Documentation

- [Getting Started Tutorial](../tutorials/getting-started.md)
- [Configuration Reference](../reference/configuration.md)
- [CLI Reference](../reference/cli.md)
- [How-To Guides](../how-to/)
- [Contributing Guide](../../CONTRIBUTING.md)
