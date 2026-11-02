# Design

## Context

See `proposal.md` for motivation. The existing cross-repository workflow checks
the provider branch against `complyctl@main`, while `ci_local.yml` builds and
tests against the dependency pinned in `go.mod`. The existing integration
target runs a test harness from the checked-out `complyctl` tree.

Upstream complyctl PR #944 owns static public API compatibility analysis for
`api/plugin` and `pkg/provider`. This change supplies the separate downstream
build and runtime validation called out by that work.

## Goals / Non-Goals

**Goals:**

- Validate provider build and integration behavior against the highest stable
  release in the major version declared by this repository's `complyctl`
  dependency.
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
the released `complyctl` source, builds providers, and runs the existing
integration target.

Keeping the jobs distinct makes the tested baseline visible and preserves the
current main-to-main signal. Parameterizing the existing job to run both
baselines would reduce YAML duplication but makes failure attribution and job
status less clear.

### Derive the supported major from go.mod

The resolver reads the direct `github.com/complytime/complyctl` requirement and
selects the highest stable semantic-version tag in that major. It rejects
malformed and prerelease tags, emits the selected tag and commit, and fails
closed when no baseline exists.

Using the declared dependency makes the check follow the compatibility promise
being validated. Selecting the unrestricted latest tag could test a future
major with intentionally incompatible APIs.

### Reuse the existing integration harness with prerequisite checks

The released checkout remains the source of the `test-cross-repo` harness. The
job verifies required scripts and build targets before running it, so older
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

## Migration Plan

1. Add the released-baseline job without removing existing CI coverage.
2. Validate a successful run against the current stable release and verify that
   the job reports its selected tag and commit.
3. Exercise failure diagnostics with controlled resolver, build, and integration
   failures where feasible.
4. Roll back by removing the new job if it blocks CI incorrectly; existing
   pinned-dependency and main-branch integration checks remain unchanged.
