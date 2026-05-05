# Contracts

The compatibility source of truth is the official `remnawave/node` 2.7.x contract:

```text
tmp/remnawave-node/libs/contract
```

Current Go structs live in:

```text
internal/contracts
```

## Golden Tests

Golden files will be stored in:

```text
testdata/contracts/official-2.7.0
```

The scaffold does not copy official TypeScript contracts into this repository. Future M0 work should extract representative request and response JSON from the official contract package and compare:

- HTTP method and path
- JSON field names
- envelope shape
- null, empty array, and empty object behavior
- date/time string formats

## Stub Policy

All planned external routes are registered now so the service does not return 404 for known Remnawave calls. Stubs must be explicit: return compatible placeholder data and keep unimplemented behavior visible in code and tests.
