# rw-node-go

`rw-node-go` is a Go implementation scaffold for a Remnawave Node-compatible service. The goal is to follow the official `remnawave/node` 2.7.x Panel-facing API while keeping the runtime small and maintainable.

This repository is currently in the framework stage. Routes, contracts, project layout, CI/CD, Docker, and documentation are in place; real Xray process control, Xray gRPC, user management, stats, torrent blocker, and nftables behavior are intentionally not implemented yet.

The HTTP layer uses Gin. `tmp/remnawave-node-go` is only a behavioral reference; this project does not follow that repository's framework layout.

## Quick Start

```sh
mise install
mise run test
mise run build
```

Run the local stub server:

```sh
NODE_PORT=2222 mise exec -- go run ./cmd/rw-node-go
```

The scaffold returns compatible JSON envelopes for planned API routes. `/node/xray/start` returns `isStarted=false` with `not implemented` until the M1 lifecycle work is implemented.

## Environment

Common variables:

```env
NODE_PORT=2222
SECRET_KEY=
XTLS_API_PORT=61000
LOG_LEVEL=info
RW_NODE_DIR=/opt/rw-node-go
INTERNAL_SOCKET_PATH=/tmp/remnawave-node.sock
INTERNAL_REST_TOKEN=
DISABLE_HASHED_SET_CHECK=false
XRAY_BIN=/usr/local/bin/xray
XRAY_CONFIG_PATH=/opt/rw-node-go/xray/config.json
XRAY_ASSET_DIR=/usr/local/share/xray
INTERNAL_REST_PORT=61001
ENABLE_UNIX_SOCKET_INTERNAL=true
ENABLE_PLUGIN_STUBS=true
```

`SECRET_KEY` parsing is scaffolded, but strict mTLS/JWT enforcement is deferred to M1.

## Images

The planned CI publishes multi-arch images to:

```text
ghcr.io/x-dora/rw-node-go
```

The current Docker image contains the `rw-node-go` binary only. It reserves the expected Xray paths but does not download or embed Xray yet.

## References

Clone reference repositories into `tmp/`:

```sh
git clone --depth 1 --branch 2.7.0 https://github.com/remnawave/node.git tmp/remnawave-node
git clone --depth 1 --branch master https://github.com/hteppl/remnawave-node-go.git tmp/remnawave-node-go
```

`tmp/` is ignored by git and Docker.
