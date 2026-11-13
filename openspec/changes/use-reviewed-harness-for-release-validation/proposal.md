# Proposal

## Why

The released-baseline job runs assertions frozen in the selected
complyctl release, including obsolete Ampel evidence filename checks.
It must instead test the unchanged released host with reviewed current
assertions, so provider compatibility measures runtime behavior rather
than historical assumptions about artifact storage.

## What Changes

- Consume the canonical complyctl harness from a reviewed immutable
  commit, independently of the selected released host commit.
- Pass the released host's binary directory through the harness's
  `COMPLYCTL_BIN_DIR` input while the harness owns its test fixtures.
- Preserve stable release resolution, checkout verification, isolated
  provider dependency replacement, all-provider builds and tests, and
  the separate main-consumer integration job.
- Report both revisions and distinguish harness preparation failures
  from released-host preparation, provider build/test, and integration
  failures. Never fall back to another host or harness on failure.
- Add offline execution tests proving the selected released binaries
  receive the integration commands and failures remain failures.
- Explicitly replace the prior release-supplied-harness design in the
  reviewed requirements. Existing diagnostic protections require
  maintainer resolution before implementation; do not bypass them.

## Capabilities

### New Capabilities

- `complyctl-release-compatibility`: Released-host validation using
  separately versioned canonical assertions. This carries forward the
  same capability from the completed, unpromoted change
  `validate-complyctl-release-compatibility`; there is no corresponding
  main spec yet. Reconcile that change before any later spec promotion.

### Modified Capabilities

None. Release compatibility is not in the main capability inventory.

## Impact

- Affects `.github/workflows/ci_cross_repo_integration.yml`, focused
  helpers and their tests under `.github/scripts/`, and harness usage
  documentation. Reuse the existing provider integration Make target.
- Depends on the merged complyctl change
  `resolve-cross-repo-evidence-mappings` tracked by
  [complyctl#970](https://github.com/complytime/complyctl/issues/970).
  That repository owns the assertions and host-selection interface.
- No provider implementation, dependency version, release resolver,
  protocol, or production source change in the selected host.
- No copied or patched historical harness, weakened assertions,
  changed timeouts, relaxed CI flags, or quality-gate modifications.
- Separate PR from `retain-scan-evidence` and provider PR #238. Merge
  the complyctl harness first, then select its verified full commit.
- Assess `CHANGELOG.md`, `AGENTS.md`, and `README.md` before closing.
