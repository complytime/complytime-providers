## Why

After the provider split (spec 004), `complytime-providers` and `complyctl` are
developed independently. The complyctl repository established the cross-repo integration
test infrastructure in its own change: a test script, test fixtures, and a CI workflow
that validates every complyctl PR against `complytime-providers@main`. However, the
reverse is not yet covered — a PR in `complytime-providers` can introduce a breaking
change that is not caught until a complyctl user encounters it at runtime.

This change adds the mirrored CI workflow to `complytime-providers`: on every PR, it
builds the Ampel provider from the PR branch, checks out `complyctl@main`, and runs
the integration test script owned by complyctl. No new test logic or fixtures are
introduced here; this change depends on the complyctl side being merged first.

## What Changes

- **New CI workflow in complytime-providers**: `.github/workflows/ci_cross_repo_integration.yml`
  triggers on PRs to main and on push to main, builds the Ampel provider from the PR branch, checks out
  `complyctl@main`, builds complyctl and the mock OCI registry, installs `snappy` and
  `ampel`, and runs `tests/cross-repo/cross_repo_integration_test.sh` from the complyctl
  checkout.

## Capabilities

### New Capabilities

- `cross-repo-integration-test`: CI validation that every `complytime-providers` PR is
  tested against `complyctl@main` using the integration test script owned by complyctl,
  ensuring the Ampel provider binary and complyctl interoperate correctly end-to-end.

### Modified Capabilities

None

## Impact

- **complytime-providers**: new CI workflow only — no source code changes.
- **complyctl**: not modified by this change. The test script and all fixtures consumed
  by this workflow live in complyctl and are maintained there.
- **Prerequisite**: The complyctl `cross-repo-integration-tests` change MUST be merged
  before this workflow is functional, as the test script it references
  (`tests/cross-repo/cross_repo_integration_test.sh`) does not exist until then.
- **Dependencies**: `snappy` and `ampel` CLI tools installed via
  `carabiner-dev/actions/install/` composite actions (same SHA pins as the complyctl
  workflow); `GITHUB_TOKEN` (standard Actions token) for snappy to read branch
  protection rules from the public `complytime/complyctl` repository.
