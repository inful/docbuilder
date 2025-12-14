# Architecture Documentation Index

This directory contains comprehensive architecture documentation for DocBuilder. The documentation is organized to provide different views and levels of detail for various audiences.

## Documentation Structure

### High-Level Overview

**[Architecture Overview (architecture.md)](architecture.md)**
- Quick reference for the staged pipeline
- Key components and responsibilities
- Namespacing and idempotence strategies
- Design rationale highlights
- **Audience:** New developers, product managers, technical leads

### Comprehensive Documentation

**[Comprehensive Architecture (comprehensive-architecture.md)](comprehensive-architecture.md)**
- Complete system architecture with all layers
- Core principles (clean architecture, event sourcing, typed state)
- Detailed package structure and responsibilities
- Data flow diagrams and sequences
- Key subsystems deep dive (themes, forges, change detection)
- Extension points and operational considerations
- Migration status summary
- **Audience:** Senior engineers, architects, contributors

**[Architecture Diagrams (architecture-diagrams.md)](architecture-diagrams.md)**
- Visual representations using ASCII and Mermaid
- System architecture diagrams
- Pipeline flow visualizations
- Package dependency graphs
- Component interaction sequences
- State machine diagrams
- Deployment architectures
- **Audience:** Visual learners, architects, documentation reviewers

**[Package Architecture Guide (package-architecture.md)](package-architecture.md)**
- Detailed package-by-package documentation
- Key types and interfaces for each package
- Usage patterns and code examples
- Design rationale for architectural decisions
- Dependency rules and import matrix
- Testing support infrastructure
- **Audience:** Contributors, maintainers, package users

### Specialized Topics

**[Namespacing Rationale (namespacing-rationale.md)](namespacing-rationale.md)**
- Forge-level namespacing design
- Collision prevention strategies
- Configuration options
- **Audience:** Users configuring multi-forge setups

**[Renderer Testing (renderer-testing.md)](renderer-testing.md)**
- Hugo rendering test strategies
- Golden test patterns
- **Audience:** Contributors working on Hugo integration

## Quick Navigation

### By Role

**New Developer Getting Started:**
1. Start with [Architecture Overview](architecture.md)
2. Review [Architecture Diagrams](architecture-diagrams.md) for visual understanding
3. Deep dive into [Package Architecture Guide](package-architecture.md) for code structure

**Contributing to Core Pipeline:**
1. Read [Comprehensive Architecture](comprehensive-architecture.md) - Pipeline Flow section
2. Study [Package Architecture Guide](package-architecture.md) - Pipeline section
3. Review [Architecture Diagrams](architecture-diagrams.md) - Pipeline Flow

**Adding New Theme Support:**
1. Read [Comprehensive Architecture](comprehensive-architecture.md) - Theme System
2. Review [Package Architecture Guide](package-architecture.md) - internal/hugo section
3. Follow [How-To Guide](../how-to/add-theme-support.md)

**Implementing Forge Integration:**
1. Read [Comprehensive Architecture](comprehensive-architecture.md) - Forge Integration
2. Study [Package Architecture Guide](package-architecture.md) - internal/forge section
3. Review [Namespacing Rationale](namespacing-rationale.md)

**Understanding State Management:**
1. Review [Comprehensive Architecture](comprehensive-architecture.md) - State Management
2. Study [Package Architecture Guide](package-architecture.md) - internal/state section
3. Check [Architecture Diagrams](architecture-diagrams.md) - State Persistence Flow

### By Topic

**Configuration System:**
- [Comprehensive Architecture](comprehensive-architecture.md#configuration--state)
- [Package Architecture Guide](package-architecture.md#internalconfig)
- [Configuration Reference](../reference/configuration.md)

**Pipeline Stages:**
- [Architecture Overview](architecture.md#pipeline-flow)
- [Architecture Diagrams](architecture-diagrams.md#pipeline-flow)
- [Package Architecture Guide](package-architecture.md#internalpipeline)

**Event Sourcing:**
- [Comprehensive Architecture](comprehensive-architecture.md#2-event-sourcing)
- [Package Architecture Guide](package-architecture.md#internaleventstore)

**Error Handling:**
- [Comprehensive Architecture](comprehensive-architecture.md#4-unified-error-handling)
- [Package Architecture Guide](package-architecture.md#internalfoundationerrors)
- [ADR-000: Uniform Error Handling](../adr/ADR-000-uniform-error-handling.md)

**Theme System:**
- [Comprehensive Architecture](comprehensive-architecture.md#1-theme-system)
- [Architecture Diagrams](architecture-diagrams.md#theme-system)
- [How-To: Add Theme Support](../how-to/add-theme-support.md)

**Change Detection:**
- [Comprehensive Architecture](comprehensive-architecture.md#3-change-detection)
- [Architecture Diagrams](architecture-diagrams.md#change-detection)
- [Package Architecture Guide](package-architecture.md#internalincremental)

**Testing Infrastructure:**
- [Package Architecture Guide](package-architecture.md#testing-support)
- [Renderer Testing](renderer-testing.md)

## Architecture Evolution

The architecture has undergone significant evolution documented in:

- **[Architecture Migration Plan](../../ARCHITECTURE_MIGRATION_PLAN.md)**
  - 19 completed phases (A-M, O-P, R-S-T-U)
  - 2 deferred phases (Q, J)
  - ~1,290 lines eliminated
  - Zero breaking changes

## Architecture Decision Records (ADRs)

For detailed architectural decisions, see the [ADR directory](../adr/):

- [ADR-000: Uniform Error Handling](../adr/ADR-000-uniform-error-handling.md)
- [ADR-001: Forge Integration Daemon](../../plan/adr-001-forge-integration-daemon.md)

## Visual Reference

### System Layers

```
┌─────────────────────────┐
│   Presentation Layer    │  cmd/docbuilder/commands, server/
├─────────────────────────┤
│   Application Layer     │  build/, services/, daemon/
├─────────────────────────┤
│     Domain Layer        │  config/, state/, docs/, hugo/, forge/
├─────────────────────────┤
│  Infrastructure Layer   │  git/, workspace/, eventstore/, auth/
├─────────────────────────┤
│   Foundation Layer      │  foundation/errors, logfields/, metrics/
└─────────────────────────┘
```

### Pipeline Stages

```
PrepareOutput → CloneRepos → DiscoverDocs → GenerateConfig →
Layouts → CopyContent → Indexes → RunHugo (optional)
```

### Key Principles

1. **Clean Architecture** - Clear dependency direction
2. **Event Sourcing** - Immutable event log
3. **Typed State** - No `map[string]any` in primary paths
4. **Unified Errors** - Single error type system
5. **Observable** - Logging, metrics, tracing

## Contributing to Architecture

When making architectural changes:

1. **Update relevant documentation** in this directory
2. **Add ADR** for significant decisions
3. **Update diagrams** if structure changes
4. **Update migration plan** if part of migration
5. **Update package guide** for API changes

## Questions and Feedback

For questions about the architecture:

- Check existing documentation first
- Review ADRs for historical context
- Open discussion in GitHub issues
- Tag architecture-related PRs with `architecture` label

## Related Documentation

- [Getting Started Tutorial](../tutorials/getting-started.md)
- [How-To Guides](../how-to/)
- [Reference Documentation](../reference/)
- [Contributing Guide](../../CONTRIBUTING.md)
- [Changelog](../../CHANGELOG.md)

---

**Last Updated:** December 2025

**Architecture Status:** Migration complete (19 phases), production-ready
