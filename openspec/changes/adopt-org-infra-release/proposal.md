## Why

`complytime-providers` is the only Go binary repo in the complytime org still using inline release preflight and GoReleaser logic (~189 lines in `release.yml`). `complyctl` and `complypack` have already adopted the reusable workflows from `complytime/org-infra`, meaning `complytime-providers` misses hardening improvements (proper semver comparison, re-run detection, configurable CI check names, skip overrides) and accumulates drift over time. Consolidating to the shared reusables reduces maintenance burden and ensures all repos benefit from preflight improvements automatically.

Ref: [complytime/complytime-providers#158](https://github.com/complytime/complytime-providers/issues/158)

## What Changes

- Replace inline `preflight` job (~120 lines of shell) with a caller to `complytime/org-infra/.github/workflows/reusable_release_preflight.yml`
- Replace inline `release` job (~40 lines of steps) with a caller to `complytime/org-infra/.github/workflows/reusable_release_goreleaser.yml`
- Configure `ci_checks` input with the two required check names: `"Build and test"` and `"Standardized CI / Run linters"`
- Reduce `release.yml` from ~189 lines to ~40 lines
- Pin reusable workflow references to a specific commit SHA (matching complyctl/complypack pattern)

## Capabilities

### New Capabilities

- `reusable-release-workflow`: Adoption of org-infra reusable release workflows (preflight + GoReleaser) replacing inline implementation

### Modified Capabilities

_None. No existing specs are affected._

## Impact

- **Files changed**: `.github/workflows/release.yml` (rewritten)
- **Dependencies**: New dependency on `complytime/org-infra` reusable workflows (already used by sibling repos)
- **CI**: No change to CI workflows (`ci_local.yml`, `ci_checks.yml` unaffected)
- **Release process**: Functionally identical from operator perspective — same `workflow_dispatch` trigger with `tag` input, same GoReleaser + cosign + syft pipeline. Gains improved semver validation and re-run detection.
- **Risk**: Low. Pattern proven in complyctl and complypack. Workflow behavior is additive (better validation), not breaking.
