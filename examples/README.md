# Examples

This directory contains example configurations and small utilities for DocBuilder.

- configs/demo-config.yaml — A complete daemon + build configuration example

Try it:

```fish
# From repo root
cp examples/configs/demo-config.yaml ./demo-config.yaml
./docbuilder build -c demo-config.yaml -v
```

Notes:

- Adjust repository URLs and tokens before running.
- Ports in the demo config: docs 8080, event receiver 8081, admin 8082.
