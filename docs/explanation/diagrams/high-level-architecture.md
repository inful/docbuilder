---
aliases:
  - /_uid/663991b1-bfe7-4c55-bd54-8f09e1120e06/
categories:
  - explanation
  - architecture
date: 2026-01-04T00:00:00Z
fingerprint: e0930feaae293ab682ab1f82f81a6fcb1727905643295ba504ced507f36078d5
lastmod: "2026-01-22"
tags:
  - architecture
  - diagrams
  - layers
title: High-Level System Architecture
uid: 663991b1-bfe7-4c55-bd54-8f09e1120e06
---

# High-Level System Architecture

This document shows the layered architecture of DocBuilder, illustrating how different components interact across layers.

**Last Updated:** January 4, 2026 - Reflects current package structure.

## Layer View

```
┌──────────────────────────────────────────────────────────────────┐
│                      COMMAND LAYER                               │
│              (cmd/docbuilder/commands/)                          │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │  Build   │  │  Daemon  │  │  Preview │  │     Discover     │  │
│  │  (Kong)  │  │ (Watch)  │  │  (Live)  │  │   (Analysis)     │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘  │
│       │             │             │                 │            │
└───────┼─────────────┼─────────────┼─────────────────┼────────────┘
        │             │             │                 │
        └─────────────┴─────────────┴─────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                    SERVICE LAYER                                │
│             (internal/build, internal/daemon)                   │
│                                                                 │
│  ┌────────────────┐  ┌─────────────────┐  ┌──────────────────┐  │
│  │ BuildService   │  │ DaemonService   │  │ DiscoveryService │  │
│  │                │  │                 │  │                  │  │
│  │ - Run()        │  │ - Start()       │  │ - Discover()     │  │
│  │ - Validate()   │  │ - Stop()        │  │ - Report()       │  │
│  └────────┬───────┘  └────────┬────────┘  └────────┬─────────┘  │
│           │                   │                    │            │
└───────────┼───────────────────┼────────────────────┼────────────┘
            │                   │                    │
            └───────────────────┴────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────────────┐
│                        PROCESSING LAYER                             │
│  (internal/hugo, internal/docs, internal/hugo/pipeline)             │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                   Hugo Generator                             │   │
│  │                                                              │   │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐     │   │
│  │  │  Pipeline   │ │   Relearn   │ │   Report Builder    │     │   │
│  │  │  Processor  │ │   Config    │ │                     │     │   │
│  │  └──────┬──────┘ └──────┬──────┘ └──────────┬──────────┘     │   │
│  └─────────┼───────────────┼───────────────────┼────────────────┘   │
│            │               │                   │                    │
│  ┌─────────▼───────────────▼───────────────────▼──────────────┐     │
│  │              Fixed Transform Pipeline                     │     │
│  │         (internal/hugo/pipeline/)                         │     │
│  │                                                          │     │
│  │  1. parseFrontMatter              - Extract YAML         │     │
│  │  2. normalizeIndexFiles           - README → _index      │     │
│  │  3. buildBaseFrontMatter          - Add defaults         │     │
│  │  4. extractIndexTitle             - H1 extraction        │     │
│  │  5. stripHeading                  - Remove H1            │     │
│  │  6. escapeShortcodesInCodeBlocks  - Escape {{ }} in ```  │     │
│  │  7. rewriteRelativeLinks          - Fix .md links        │     │
│  │  8. rewriteImageLinks             - Fix image paths      │     │
│  │  9. generateFromKeywords          - Create from @keywords│     │
│  │  10. addRepositoryMetadata        - Inject repo info     │     │
│  │  11. addEditLink                  - Generate editURL     │     │
│  │  12. serializeDocument            - Output YAML + content│     │
│  └──────────────────────────────────────────────────────────┘     │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
┌─────────────────────────────────▼────────────────────────────────┐
│                          DOMAIN LAYER                            │
│  (internal/config, internal/state, internal/docs)                │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │  Config  │  │  State   │  │ DocFile  │  │  Repository      │  │
│  │          │  │          │  │          │  │                  │  │
│  │ - Hugo   │  │ - Git    │  │ - Path   │  │ - URL            │  │
│  │ - Build  │  │ - Docs   │  │ - Trans  │  │ - Branch         │  │
│  │ - Forge  │  │ - Build  │  │   forms  │  │ - Auth           │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘  │
└───────┼─────────────┼─────────────┼─────────────────┼────────────┘
        │             │             │                 │
        └─────────────┴─────────────┴─────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                    INFRASTRUCTURE LAYER                         │
│  (internal/git, internal/forge, internal/workspace)             │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────────┐  │
│  │   Git    │  │  Forge   │  │ Event    │  │  Workspace      │  │
│  │  Client  │  │ Clients  │  │ Store    │  │  Manager        │  │
│  │          │  │          │  │          │  │                 │  │
│  │ - Clone  │  │ - GitHub │  │ - Append │  │ - Create()      │  │
│  │ - Update │  │ - GitLab │  │ - Query  │  │ - Cleanup()     │  │
│  │ - Auth   │  │ - Forgejo│  │          │  │                 │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬────────┘  │
│       │             │             │                 │           │
│       └─────────────┴─────────────┴─────────────────┘           │
│                             │                                   │
│                    ┌────────▼────────┐                          │
│                    │   Foundation    │                          │
│                    │     Errors      │                          │
│                    │                 │                          │
│                    │ - ClassifiedErr │                          │
│                    │ - Categories    │                          │
│                    │ - Retry Logic   │                          │
│                    └─────────────────┘                          │
└─────────────────────────────────────────────────────────────────┘
```

## Layer Responsibilities

### Command Layer
- **Purpose**: User interface and CLI command handling
- **Components**: Build, Daemon, Preview, Discover commands
- **Dependencies**: Service layer only
- **Example**: `cmd/docbuilder/commands/build.go` parses CLI flags and delegates to BuildService

### Service Layer
- **Purpose**: Business logic orchestration and workflow management
- **Components**: BuildService, DaemonService, DiscoveryService
- **Dependencies**: Processing and domain layers
- **Example**: `internal/build/default_service.go` orchestrates the build pipeline

### Processing Layer
- **Purpose**: Content transformation and Hugo site generation
- **Components**: Hugo Generator, Pipeline Processor, Relearn Configuration
- **Dependencies**: Domain and infrastructure layers
- **Key Feature**: Fixed 12-step transform pipeline (ADR-003)

### Domain Layer
- **Purpose**: Core business entities and rules
- **Components**: Config, State, DocFile, Repository models
- **Dependencies**: Foundation layer only
- **Characteristic**: Pure domain logic, no infrastructure concerns

### Infrastructure Layer
- **Purpose**: External system integration
- **Components**: Git client, Forge clients, Event store, Workspace management
- **Dependencies**: Foundation layer only
- **Example**: `internal/git/git.go` handles repository cloning with authentication

### Foundation Layer
- **Purpose**: Cross-cutting concerns and shared utilities
- **Components**: Unified error handling, logging fields, metrics
- **Dependencies**: None (leaf layer)
- **Key Package**: `internal/foundation/errors` provides classified error system

## Key Design Principles

1. **Dependency Rule**: Dependencies flow downward only; upper layers depend on lower layers, never the reverse
2. **Single Responsibility**: Each layer has a distinct, well-defined purpose
3. **Testability**: Lower layers (domain, infrastructure) are easily testable in isolation
4. **Error Propagation**: All errors use `internal/foundation/errors` for consistent classification and context

## References

- [Architecture Overview](../architecture.md)
- [Comprehensive Architecture](../comprehensive-architecture.md)
- [Package Dependencies Diagram](package-dependencies.md)
- [Pipeline Flow Diagrams](pipeline-flow.md)
