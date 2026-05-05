# Development

## Setup

Initialize references:

```sh
git clone --depth 1 --branch 2.7.0 https://github.com/remnawave/node.git tmp/remnawave-node
git clone --depth 1 --branch master https://github.com/hteppl/remnawave-node-go.git tmp/remnawave-node-go
```

Do not edit `tmp/`; refresh it from upstream when needed.

## Commands

```sh
mise run fmt
mise run test
mise run build
mise run docker-build
```

`mise run lint` expects `golangci-lint` to be installed locally.

## Local Server

```sh
NODE_PORT=2222 mise exec -- go run ./cmd/rw-node-go
```

The framework starts an HTTP stub server. mTLS/JWT enforcement is not active until M1.

## Docker

```sh
docker build -t ghcr.io/x-dora/rw-node-go:local .
```

The first Docker stage compiles Go. The runtime stage is Alpine and reserves Xray paths for future lifecycle work.
