---
title: API Reference
description: API documentation and reference
weight: 2
---

# API Reference

Complete API documentation for developers.

## Core API

### BuildService

The main service for building documentation sites.

**Methods:**

- `Build(ctx context.Context, outputDir string) error` - Builds the documentation site
- `Validate(ctx context.Context) error` - Validates the configuration

### Configuration

Configuration structure:

```go
type Config struct {
    Repositories []Repository
    Hugo         HugoConfig
    Output       OutputConfig
}
```

## Examples

```go
// Create a new build service
cfg, _ := config.Load("config.yaml")
svc := build.NewDefaultService(cfg)

// Execute the build
err := svc.Build(context.Background(), "./output")
if err != nil {
    log.Fatal(err)
}
```
