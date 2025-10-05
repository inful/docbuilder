# Examples

This directory contains example configurations and small utilities for DocBuilder.

- configs/demo-config.yaml — A complete daemon + build configuration example
- configs/hextra-config.yaml — Hextra theme local test config
- configs/config-v2-test.yaml — V2 daemon configuration for local testing
- configs/git-home-config.yaml — Example pointing at a self-hosted Git service
- tools/debug_webhook.go — A small tool to exercise event receiver helpers

Try it:

```fish
# From repo root
cp examples/configs/demo-config.yaml ./demo-config.yaml
./docbuilder build -c demo-config.yaml -v
```

Notes:

- Adjust repository URLs and tokens before running.
- Ports in the demo config: docs 8080, event receiver 8081, admin 8082.
