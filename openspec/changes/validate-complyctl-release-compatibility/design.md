# Design

## Context

See `proposal.md` for motivation. The existing cross-repository workflow checks
the provider branch against `complyctl@main`, while `ci_local.yml` builds and
tests against the dependency pinned in `go.mod`. The existing integration
target runs a test harness from the checked-out `complyctl` tree.

Upstream complyctl PR #944 owns static public API compatibility analysis for
`api/plugin` and `pkg/provider`. This change supplies the separate downstream
build and runtime validation called out by that work.

## Upstream Verification

On 2026-11-03, `gh pr view 944 --repo complytime/complyctl` reported that PR
#944 merged at `b6aebd721deebf05d48a17c8e5fb25b3c2e271ee`. Its merged summary
retains downstream provider build and runtime validation for
`complytime-providers#187`, while complyctl owns static public API compatibility
analysis. Task 1.1 is complete.

## Released Harness Verification

On 2026-11-06, the direct dependency in this repository's `go.mod` was
`github.com/complytime/complyctl v1.1.0`. The release resolver selected
`v1.2.0`, published on 2026-11-06, as the highest stable tag in the required
v1 compatibility line. The tag resolves to commit
`f10c1426f3674bbcb967a8aad8ca529b32e21212`.

The `v1.2.0` source tree contains the required release-side interface:

- `Makefile` defines `build`, which compiles `./cmd/...` into `bin/`, including
  `complyctl` and `mock-oci-registry`.
- `Makefile` defines `test-cross-repo`, which builds complyctl and invokes the
  executable `tests/cross-repo/cross_repo_integration_test.sh`.
- The harness and its Ampel/OPA fixtures are present at
  `tests/cross-repo/`. The harness requires `PROVIDERS_BIN_DIR`,
  `GITHUB_TOKEN`, `conftest`, `jq`, and `curl`, and verifies that the built
  complyctl, mock registry, Ampel provider, and OPA provider are executable.
- The release declares Go `1.26.8`; the existing workflow selects the higher
  Go version from both repositories, so the providers' declared Go `1.26.10`
  remains suitable.

The current integration workflow meets the harness's non-runner
prerequisites: it builds both repositories, installs Snappy, Ampel, and
conftest, and provides `PROVIDERS_BIN_DIR` plus `GITHUB_TOKEN`. Therefore,
`v1.2.0` can provide the required build target and test harness. The released
baseline job must check all prerequisites, including `jq` and `curl`, so a
future selected release fails with a preparation diagnostic rather than a shell
error.

## Goals / Non-Goals

**Goals:**

- Validate provider build and integration behavior against the highest stable
  release in the compatibility line declared by this repository's `complyctl`
  dependency: its major version for v1+ or its minor version for v0.
- Keep the existing `complyctl@main` integration path intact.
- Produce diagnostics that identify the selected release and the stage that
  failed.

**Non-Goals:**

- Reimplement Buf or `apidiff` public API checks from complyctl PR #944.
- Test prerelease, arbitrary historical, or future-major `complyctl` versions.
- Change provider APIs, dependency versions, or the upstream release process.

## Decisions

### Add a distinct released-baseline integration job

Add a job alongside the current main-branch integration job. It resolves the
stable baseline and passes the tag and commit to a consumer job that checks out
the resolved commit from the released `complyctl` source. The consumer creates
an isolated provider checkout, temporarily replaces its `complyctl` module
dependency with that checkout, then builds all provider binaries and runs the
existing Ampel-focused integration target. The replacement is confined to the
CI workspace and does not change the repository's committed dependency.

Keeping the jobs distinct makes the tested baseline visible and preserves the
current main-to-main signal. Parameterizing the existing job to run both
baselines would reduce YAML duplication but makes failure attribution and job
status less clear. The isolated replacement makes the provider compile against
the selected release rather than its normally pinned dependency.

### Derive the supported major from go.mod

The resolver reads the direct `github.com/complytime/complyctl` requirement and
selects the highest stable semantic-version tag in its compatibility line: the
same major for v1+ dependencies or the same minor for v0 dependencies. It
rejects malformed and prerelease tags, emits the selected tag and commit, and
fails closed when no baseline exists.

The resolver uses `go mod edit -json` and `jq` to read only direct entries in
`Require`. It ignores `Exclude` and `Replace` entries and reports malformed
module files with a `release-baseline:` diagnostic.

Using the declared dependency makes the check follow the compatibility promise
being validated. Selecting the unrestricted latest tag could test a future
major, or a breaking v0 minor, with intentionally incompatible APIs.

### Reuse the existing integration harness with prerequisite checks

The released checkout remains the source of the `test-cross-repo` harness. The
job verifies that its checkout HEAD equals the resolved commit, verifies required
scripts and build targets, and invokes its `make test-cross-repo` target. This
ensures the selected release supplies both the CLI and harness, while older
releases fail with a clear preparation diagnostic instead of an ambiguous shell
failure.

Copying the harness into this repository would decouple it from released
complyctl, but would duplicate cross-repository test ownership and risk drift.

### Keep failure boundaries explicit

Separate workflow steps resolve the baseline, prepare the checkout, build
providers, and run integration. Each step names its purpose and leaves command
output intact. The job summary records the resolved tag and commit.

This is preferred over a single shell script because GitHub Actions exposes
step-level failures directly to maintainers.

## Risks / Trade-offs

- [A selected older release lacks the integration harness] -> Check required
  prerequisites explicitly and report the release tag and missing item.
- [Tag selection changes as new releases are published] -> Record the selected
  tag and commit in each CI run; reproduce failures with those values.
- [Release resolution depends on network access] -> Fail closed with a
  release-baseline diagnostic rather than silently skipping validation.
- [PR #944 merges differently from its current design] -> Verify its merged
  API-compatibility policy and downstream handoff before implementation.

## Local Verification

On 2026-11-02, the provider checks completed successfully after the workflow
change:

- `make build` exited 0 and built `complyctl-provider-openscap`,
  `complyctl-provider-ampel`, and `complyctl-provider-opa`.
- `make test` exited 0. All 30 packages completed without failures; the three
  provider command packages reported no test files.
- `make lint` exited 0 with `0 issues.`
- All three released-baseline shell test scripts exited 0: resolver,
  released-checkout prerequisite, and workflow diagnostics.
- `yamllint .github/workflows/ci_cross_repo_integration.yml` exited 0.

On 2026-11-06, CI run `42226391757` passed the released-baseline resolution and
integration jobs at provider commit `9f7cdd44787c770257ff3174e35791dea57d0bec`.
Its report step published tag `v1.2.0` and commit
`f10c1426f3674bbcb967a8aad8ca529b32e21212`. The CI actionlint check also passed.
This supplies the CI reporting evidence for task 2.4.

After the review fixes on 2026-11-06, local validation passed:

- All three released-baseline shell test scripts, including regression cases
  for exclusion-only, replacement-only, indirect, and malformed module files.
- `shellcheck .github/scripts/*.sh .github/scripts/test/*.sh`.
- `yamllint .github/workflows/ci_cross_repo_integration.yml` and actionlint
  `v1.7.9` on the same workflow.
- `make build`, `make test`, and `make lint`.
- The configured Renovate regex extracts exactly one Conftest pin from the
  workflow. Version updates require a matching archive checksum before merge.

These are local results; the modified working tree has not run in GitHub CI.

## Migration Plan

1. Add the released-baseline job without removing existing CI coverage.
2. Validate a successful run against the current stable release and verify that
   the job reports its selected tag and commit.
3. Exercise failure diagnostics with controlled resolver, build, and integration
   failures where feasible.
4. Roll back by removing the new job if it blocks CI incorrectly; existing
   pinned-dependency and main-branch integration checks remain unchanged.
