## Why

The ampel provider reads granular policy JSON files from a hardcoded local directory (`.complytime/ampel/granular-policies/`), with an optional override via `ampel_policy_dir`. With `ComplypackContentPath` now available in `GenerateRequest` (complytime/complyctl#536), the provider should consume complypack content when available, matching the pattern already implemented in the OPA provider. This enables portable, versioned policy distribution through complypacks instead of requiring policies to be pre-staged on the filesystem. Ref: [complytime/complytime-providers#39](https://github.com/complytime/complytime-providers/issues/39).

## What Changes

- Add complypack content path resolution to the ampel provider's `Generate()` method with precedence: `ComplypackContentPath` > `ampel_policy_dir` variable > default path
- Add a secure tar.gz extraction helper with zip-slip protection, symlink rejection, and size limits (ported from the OPA provider's `extractTarGz`)
- Add a `resolveComplypackPath()` function that handles both directory and tar.gz archive inputs with idempotent extraction
- Add unit tests for complypack resolution, tar extraction (happy path, backward compat, malicious tar, missing file), and Generate integration with complypack paths

## Capabilities

### New Capabilities
- `complypack-content`: Resolve and extract complypack content archives for granular policy loading in the ampel provider

### Modified Capabilities

## Impact

- `cmd/ampel-provider/server/server.go` -- `Generate()` method gains complypack resolution logic
- New file: `cmd/ampel-provider/server/unpack.go` -- secure tar extraction utilities
- `cmd/ampel-provider/server/server_test.go` -- new Generate tests with complypack paths
- New file: `cmd/ampel-provider/server/unpack_test.go` -- extraction unit tests
- Dependency: requires vendored complyctl with `ComplypackContentPath` field (complytime/complytime-providers#38)
