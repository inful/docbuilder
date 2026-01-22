---
aliases:
  - /_uid/b43f4ed6-21cb-4a80-9cdd-3304d03cca05/
categories:
  - explanation
date: 2026-01-04T00:00:00Z
fingerprint: e11bd76b4727ee2e3bd196f5ed2a233d569d04f5d1f1d1f0a7dcf4a4e16ff2c1
lastmod: "2026-01-22"
tags:
  - architecture
  - diagrams
  - visualization
title: Architecture Diagrams Index
uid: b43f4ed6-21cb-4a80-9cdd-3304d03cca05
---

# Architecture Diagrams Index

This directory contains comprehensive visual representations of DocBuilder's architecture. Each diagram set has been verified against the current codebase implementation.

**Last Updated:** January 4, 2026 - All diagrams verified and split into focused documents.

## Overview

The architecture diagrams are organized into separate documents by category:

1. **[High-Level System Architecture](diagrams/high-level-architecture.md)** - Layered architecture view
2. **[Pipeline Flow](diagrams/pipeline-flow.md)** - Sequential stage execution and detailed stage operations
3. **[Package Dependencies](diagrams/package-dependencies.md)** - Import relationships and dependency rules
4. **[Data Flow](diagrams/data-flow.md)** - Configuration, build execution, and state persistence flows
5. **[Component Interactions](diagrams/component-interactions.md)** - Theme config, forge integration, change detection
6. **[State Machines](diagrams/state-machines.md)** - Build, repository, and theme configuration state transitions

## Quick Navigation

### By Use Case

**Understanding the System**:
- Start with [High-Level System Architecture](diagrams/high-level-architecture.md) for overall structure
- Then review [Package Dependencies](diagrams/package-dependencies.md) for code organization

**Build Pipeline Development**:
- Review [Pipeline Flow](diagrams/pipeline-flow.md) for stage details
- Check [Data Flow](diagrams/data-flow.md) for data movement
- Reference [State Machines](diagrams/state-machines.md) for state transitions

**Integration Work**:
- See [Component Interactions](diagrams/component-interactions.md) for forge and theme integration
- Review [Data Flow](diagrams/data-flow.md) for configuration and metadata handling

**Debugging**:
- Check [State Machines](diagrams/state-machines.md) for valid state transitions
- Review [Pipeline Flow](diagrams/pipeline-flow.md) for stage execution order
- See [Data Flow](diagrams/data-flow.md) for data transformation steps

### By Component

**Hugo Generator**:
- [Pipeline Flow: CopyContent Stage](diagrams/pipeline-flow.md#stage-detail-copycontent)
- [Component Interactions: Relearn Theme](diagrams/component-interactions.md#relearn-theme-configuration)
- [High-Level: Processing Layer](diagrams/high-level-architecture.md#layer-view)

**Git Operations**:
- [Data Flow: Repository Metadata](diagrams/data-flow.md#repository-metadata-flow)
- [State Machines: Repository State](diagrams/state-machines.md#repository-state-machine)
- [Component Interactions: Change Detection](diagrams/component-interactions.md#change-detection-system)

**Build Service**:
- [Pipeline Flow: Sequential Execution](diagrams/pipeline-flow.md#sequential-stage-execution)
- [State Machines: Build State](diagrams/state-machines.md#build-state-machine)
- [Data Flow: Build Execution](diagrams/data-flow.md#build-execution)

**Forge Integration**:
- [Component Interactions: Forge Integration](diagrams/component-interactions.md#forge-integration)
- [Data Flow: Repository Metadata](diagrams/data-flow.md#repository-metadata-flow)

## Diagram Verification Status

All diagrams have been verified against the current codebase (January 4, 2026):

| Diagram Set | Verification Status | Key Updates |
|-------------|-------------------|-------------|
| High-Level Architecture | ✅ Verified | Updated to 12-step transform pipeline |
| Pipeline Flow | ✅ Verified | Corrected stage names and execution order |
| Package Dependencies | ✅ Verified | Confirmed import rules and layer boundaries |
| Data Flow | ✅ Verified | Validated against current implementation |
| Component Interactions | ✅ Verified | Updated for Relearn-only configuration |
| State Machines | ✅ Verified | Confirmed state transitions and triggers |

## Key Architecture Changes Reflected

These diagrams reflect the following architectural decisions:

1. **ADR-003**: Fixed 12-step transform pipeline (not 11)
2. **Relearn Theme Only**: No multi-theme system, hardcoded Relearn defaults
3. **Unified Error Handling**: All errors use `internal/foundation/errors`
4. **Layer Architecture**: Strict dependency flow (command → service → domain → infrastructure → foundation)
5. **State Decomposition**: BuildState split into GitState, DocsState, PipelineState

## Diagram Conventions

**Layer Diagrams**:
- Boxes represent components
- Arrows show dependencies (A → B means "A depends on B")
- Layers are horizontal (upper layers depend on lower layers)

**Flow Diagrams**:
- Rectangles represent processes/stages
- Diamonds represent decisions
- Arrows show execution flow
- Dashed lines show optional flows

**State Machines**:
- Circles/rounded boxes represent states
- Arrows show transitions
- Notes explain transition conditions

**Sequence Diagrams**:
- Vertical lines represent components
- Horizontal arrows show interactions
- Time flows top-to-bottom

## Maintenance

When updating architecture:

1. **Check All Affected Diagrams**: Changes may impact multiple diagram sets
2. **Verify Implementation**: Ensure diagrams match actual code behavior
3. **Update Verification Date**: Update "Last Updated" timestamp
4. **Cross-Reference**: Update related documentation links

**Common Update Triggers**:
- Adding/removing pipeline stages
- Changing package dependencies
- Adding new components
- Modifying state transitions
- Changing configuration structure

## Related Documentation

- [Comprehensive Architecture](comprehensive-architecture.md) - Detailed architecture guide
- [Architecture Overview](architecture.md) - High-level architecture concepts
- [Package Architecture Guide](package-architecture.md) - Package-level documentation
- [Namespacing Rationale](namespacing-rationale.md) - Design decisions for namespace structure

