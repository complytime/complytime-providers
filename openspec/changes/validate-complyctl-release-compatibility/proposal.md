# Proposal

## Why

This repository currently builds and tests against its pinned `complyctl`
dependency and runs cross-repository integration against `complyctl@main`.
Neither check validates provider behavior against the latest stable release in
the supported major version, leaving a compatibility gap when the dependency
is updated.

Upstream `complyctl` PR #944 adds provider-facing API compatibility checks and
explicitly retains downstream build and runtime validation as this repository's
responsibility. This change closes that downstream gap without duplicating the
upstream static API checks.

## What Changes

- Add CI coverage that runs the provider integration workflow against a stable
  `complyctl` release in the supported major version.
- Preserve the existing `make build`, `make test`, and `complyctl@main`
  integration coverage.
- Report the selected release tag and commit, and distinguish release
  resolution, build, and integration failures.
- Document the validation boundary: upstream owns public API compatibility
  analysis; this repository owns downstream provider build and runtime checks.

## Capabilities

### New Capabilities
- `complyctl-release-compatibility`: Continuous validation that provider
  binaries interoperate with the selected stable `complyctl` release.

### Modified Capabilities

None.

## Impact

- Affects `.github/workflows/ci_cross_repo_integration.yml` and any focused
  workflow helper required to resolve the release baseline.
- Reuses `make build` and `make test-cross-repo` rather than changing provider
  APIs or adding runtime dependencies.
- Depends on the upstream compatibility policy introduced by complyctl PR #944;
  implementation must verify its merged behavior before relying on it.
