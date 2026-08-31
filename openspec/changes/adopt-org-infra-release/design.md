## Context

`complytime-providers` releases via a manually-triggered `release.yml` workflow (~189 lines) that implements preflight validation and GoReleaser execution inline. The `complytime/org-infra` repository provides reusable versions of both workflows (`reusable_release_preflight.yml` and `reusable_release_goreleaser.yml`) that are already adopted by `complyctl` and `complypack`. The inline implementation has known limitations: `sort -V` for semver ordering (breaks on pre-release tags), hardcoded CI check names, no skip override inputs, and no smart re-run detection.

The current `release.yml` has two jobs:
- `preflight`: Tag format validation, uniqueness check, semver ordering, CI status verification, unreleased commit check, tag creation/push
- `release`: Checkout at tag, setup Go, install cosign + syft, run GoReleaser

## Goals / Non-Goals

**Goals:**
- Replace inline `preflight` and `release` jobs with callers to org-infra reusable workflows
- Pin reusable workflow references to commit SHA `0c784711926c9864f027ec565fd7c06a382d80f8` (v0.7.1), matching sibling repos
- Provide explicit `ci_checks` input with `["Build and test", "Standardized CI / Run linters"]` to avoid auto-discovery dependency on `yq`
- Preserve the `workflow_dispatch` trigger with `tag` input
- Expose `skip_semver_check`, `skip_ci_checks`, and `skip_unreleased_check` override inputs on `workflow_dispatch` and forward them to the reusable preflight, matching the complyctl pattern
- Use explicit top-level `permissions: contents: none / id-token: none` with job-level permissions (least-privilege, matching complyctl)
- Verify `.goreleaser.yaml` compatibility with the reusable GoReleaser workflow

**Non-Goals:**
- Modifying `.goreleaser.yaml` configuration
- Changing CI workflows (`ci_local.yml`, `ci_checks.yml`)
- Adding pre-release support (`allow_prerelease` stays at default `false`)
- Adding `tag_push_token` secret (existing `GITHUB_TOKEN` is sufficient since providers does not need tag events to trigger downstream workflows)

## Decisions

### D1: Pin to SHA, not tag

Pin reusable workflow references to commit SHA `0c784711926c9864f027ec565fd7c06a382d80f8` (v0.7.1) rather than a mutable tag. This matches the pattern used by complyctl and complypack, and prevents supply chain attacks via tag mutation in the org-infra repo.

**Alternative**: Pin to `@v0.7.1` tag — rejected because GitHub Actions best practice recommends SHA pinning for third-party (and cross-repo) actions.

### D2: Explicit ci_checks over auto-discovery

Pass `ci_checks: '["Build and test", "Standardized CI / Run linters"]'` explicitly rather than relying on auto-discovery. Auto-discovery requires `yq` to be available on the runner and parses workflow YAML at runtime, introducing fragility. The check names are stable (derived from `ci_local.yml` job name and `ci_checks.yml` calling the reusable linter).

**Alternative**: Omit `ci_checks` and rely on auto-discovery — rejected because it adds runtime dependency on `yq` and makes the check names implicit.

### D3: No secrets.tag_push_token

The providers repo does not need tag-push events to trigger downstream workflows, so the default `GITHUB_TOKEN` is sufficient for tag creation. This avoids creating and managing an additional secret.

### D4: Remove concurrency from caller

The reusable preflight workflow defines its own `concurrency` group (`release-${{ github.repository }}`). The caller does not need a duplicate concurrency block.

### D5: Expose skip override inputs

Forward `skip_semver_check`, `skip_ci_checks`, and `skip_unreleased_check` as `workflow_dispatch` boolean inputs (default `false`) and pass them to the reusable preflight. This matches complyctl's implementation and provides operational escape hatches for exceptional situations (e.g., hotfix releases).

### D6: Explicit deny top-level permissions

Use `permissions: contents: none / id-token: none` at the workflow level rather than `permissions: {}`. Both are functionally equivalent, but the explicit deny matches complyctl's style and communicates intent more clearly.

## Risks / Trade-offs

- **[Upstream drift]** → Future org-infra changes could break providers. Mitigated by SHA pinning; upgrades are explicit and deliberate.
- **[CI check name coupling]** → If `ci_local.yml` or `ci_checks.yml` job names change, the explicit `ci_checks` input must be updated. Mitigated by the fact that check name changes are rare and would be caught immediately on the next release attempt.
- **[Reduced local visibility]** → Inline logic is easier to read without navigating to another repo. Mitigated by the workflow file having clear comments pointing to the reusable source, and the org-infra workflows being well-documented.
