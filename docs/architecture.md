# Architecture

The scaffold keeps Remnawave-facing API compatibility separate from runtime implementation details.

## Layers

- `cmd/rw-node-go`: process entrypoint, configuration loading, runtime state creation, and HTTP server startup.
- `internal/httpapi`: Gin HTTP server, route registration, response envelope helpers, and future TLS/JWT/zstd middleware.
- `internal/contracts`: public request and response structs for the Panel-facing API.
- `internal/controller`: route handlers. Current handlers are compatible stubs.
- `internal/state`: in-memory runtime state for Xray status, hashes, inbound users, and plugin state.
- `internal/xray`: future external Xray process and gRPC control abstraction.
- `internal/system`: future system stats, network capability, conntrack, and nftables integration.
- `internal/plugin`: future torrent blocker and nftables plugin behavior.

## Runtime Direction

M1 will keep Xray as an external process. The `xray.Core` interface is the boundary that later allows an embedded core experiment without changing public API handlers.

## API Envelope

External Remnawave routes return:

```json
{
  "response": {}
}
```

Internal debug routes may return direct JSON objects because they are not Panel-facing contracts.
