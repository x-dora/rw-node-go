# Agent Instructions

This repository is a Go implementation scaffold for a Remnawave Node-compatible service. The target contract is the official `remnawave/node` 2.7.x Panel-facing API.

## Current Boundary

- Framework first: keep public routes, contracts, CI/CD, Docker, and docs in place.
- Internal behavior is intentionally stubbed until the M1-M6 milestones in `REMNAWAVE_NODE_GO_PLAN.md`.
- Do not present stub behavior as a real Xray, stats, plugin, nftables, or conntrack implementation.

## Required References

- `REMNAWAVE_NODE_GO_PLAN.md` is the local implementation plan.
- `tmp/remnawave-node/libs/contract` is the official contract reference after cloning `https://github.com/remnawave/node` at tag `2.7.0`.
- `tmp/remnawave-node-go` is the community Go reference after cloning `https://github.com/hteppl/remnawave-node-go` at `master`.
- Do not modify files under `tmp/`; they are local reference checkouts only.
- Do not copy the community Go implementation's framework layout. This project uses its own layout and Gin for HTTP routing.

## Coding Rules

- Keep public JSON field names, route paths, HTTP methods, and response envelopes stable.
- Prefer the Go standard library and small, justified dependencies.
- Keep changes scoped and easy to review.
- Never log `SECRET_KEY`, JWTs, node private keys, client certificates, or bearer tokens.
- Keep `XTLS_API_PORT` and internal REST/socket endpoints local-only.
- Do not expose Xray gRPC, internal REST, or Unix socket paths through Docker publish examples.

## Tests

- Add or update tests when adding a route, public contract struct, or response shape.
- Real behavior added after the scaffold must include integration tests where practical.
- Contract changes should be checked against official `remnawave/node` contracts and golden files.

## Commands

- `mise run fmt`
- `mise run test`
- `mise run build`
- `mise run docker-build`
