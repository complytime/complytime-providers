# ADR-0001: OPA Provider Scan Architecture Redesign

**Status**: Accepted
**Date**: 2026-05-12
**Decision makers**: Fabian Ortiz, jpower432 (reviewer)

## Context

PR #12 introduced the OPA/conftest compliance provider (`cmd/opa-provider/`). During review, three interconnected design issues were identified in the scan pipeline:

1. **Data loading is tightly coupled to `server.go`**. The `scanRemoteBranch()` and `processLocalTarget()` methods handle git cloning, local path resolution, and policy evaluation inline. Additionally, `buildTokenEnv()` in `scan/scan.go` sets `GITHUB_TOKEN` as an environment variable, but `git clone` over HTTPS does not read this variable for authentication — the current git auth mechanism is broken.

2. **Error semantics are misleading**. `CheckTools()` returns `([]string, error)` but the error is always nil — `exec.LookPath` errors are converted to "missing" entries. `WritePerTargetResult()` returns errors that all three call sites log and discard. `Scan()` absorbs all per-target errors into result structs and returns nil, so the caller (complyctl) cannot distinguish a clean scan from one with failures via the error channel or response status.

3. **Bundle resolution uses first-match**. `extractBundleRef()` takes the first `opa_bundle_ref` found across all targets. If different targets require different policy bundles, they are silently scanned with the wrong bundle.

These three concerns are related: the DataLoader refactor naturally provides the structure to fix error propagation and per-target bundle handling.

## Decision

### DataLoader Interface

Extract data loading behind a `DataLoader` interface:

```go
type DataLoader interface {
    Load(target provider.Target, workDir string) (inputPath string, err error)
}
```

Two initial implementations:
- **`GitLoader`**: Clones remote repositories. Uses a git credential helper for HTTPS authentication instead of setting `GITHUB_TOKEN` as an environment variable.
- **`LocalPathLoader`**: Validates and resolves local filesystem paths.

`Scan()` delegates to the appropriate loader based on target variables, then evaluates policies against the loaded input. This separates data acquisition from policy evaluation.

### Error Handling

- Remove the unused `error` return from `CheckTools()` — its signature becomes `func CheckTools() []string`.
- Add a scan-level status or error field to the scan response so complyctl can determine overall scan health without inspecting each individual assessment.
- `WritePerTargetResult()` error handling remains log-and-continue (write failures are non-fatal), but call sites should use consistent handling rather than mixing patterns. **Amended by [ADR-0002](0002-error-propagation-and-target-variable-constants.md)**: write errors are now propagated via `errors.Join` instead of logged and discarded.

### Per-Target Bundles

Each target can specify its own `opa_bundle_ref`. The provider pulls and caches bundles as needed, keyed by bundle reference. If a bundle has already been pulled for a prior target in the same scan request, it is reused from cache. `extractBundleRef()` is removed in favor of per-target bundle resolution within the scan loop.

## Alternatives Considered

### Alternative 1: Keep inline data loading, fix only the auth bug

Fix `buildTokenEnv()` to use a git credential helper and leave the rest of the architecture unchanged. This is the minimal change to address the broken auth.

Rejected because it does not address the testability issue (mocking git operations requires the `ScanRunner` global), does not fix the misleading error returns, and does not solve the per-target bundle problem. The reviewer's feedback indicates these are design-level issues, not isolated bugs.

### Alternative 2: Token embedded in clone URL

Rewrite the clone URL to `https://<token>@github.com/...` instead of using a credential helper. This is simpler to implement.

Rejected because the token appears in process listings (`ps aux`), git config, and potentially in error messages or logs. The credential helper approach keeps tokens out of the process table and URL strings.

### Alternative 3: GIT_ASKPASS for auth

Set `GIT_ASKPASS` to a helper script or binary that returns the token when invoked by git. Secure and does not leak to the process table.

Not chosen as the primary approach because it requires writing a temporary script or binary to disk and managing its lifecycle. The git credential helper achieves the same security without the complexity of a temporary file. However, this remains a viable fallback if credential helper integration proves difficult on certain platforms.

### Alternative 4: Single bundle with conflict validation

Keep one bundle per scan request but fail if targets specify conflicting `opa_bundle_ref` values. Simpler than per-target bundles and prevents silent misconfiguration.

Not chosen because per-target bundles are the more flexible design. Different repositories may legitimately require different policy sets (e.g., infrastructure repos vs application repos in the same scan request). Restricting to a single bundle would force users to split scan requests unnecessarily.

## Consequences

### Positive

- **Testability**: `DataLoader` interface enables unit testing of `Scan()` without mocking git or filesystem operations. Test doubles implement `DataLoader` directly.
- **Auth fix**: Git credential helper resolves the broken HTTPS authentication for private repositories.
- **Accurate error reporting**: Callers can distinguish clean scans from scans with failures via the response status field.
- **Flexibility**: Per-target bundles support heterogeneous scan requests without requiring users to split targets across multiple invocations.
- **Reduced dead code**: Removing the always-nil error from `CheckTools()` eliminates an unreachable code path in `checkRequiredTools()`.

### Negative

- **Interface overhead**: The `DataLoader` interface adds a level of indirection for what is currently straightforward inline code. This is acceptable given the testability and auth benefits.
- **Bundle caching complexity**: Per-target bundle resolution requires tracking which bundles have been pulled during a scan request to avoid redundant downloads.
- **complyctl coordination**: Adding a scan-level status field to the response may require changes to the `provider.ScanResponse` struct in complyctl upstream. This must be coordinated to avoid breaking the provider interface contract.

### Risks

- **complyctl interface stability**: If `provider.ScanResponse` cannot be extended without a breaking change, the error signaling improvement may need to be deferred or implemented differently (e.g., using the existing assessment log with a synthetic `scan-status` requirement ID).
- **Credential helper portability**: Git credential helper behavior varies across platforms (macOS Keychain, Linux libsecret, Windows Credential Manager). The `GitLoader` implementation must handle cases where no credential helper is configured.
- This decision should be revisited if complyctl introduces a native mechanism for per-target bundle specification or scan-level status reporting that supersedes these changes.
