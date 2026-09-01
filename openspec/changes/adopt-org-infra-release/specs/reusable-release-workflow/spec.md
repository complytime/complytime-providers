## ADDED Requirements

### Requirement: Release workflow uses org-infra reusable preflight
The `release.yml` workflow SHALL invoke `complytime/org-infra/.github/workflows/reusable_release_preflight.yml` as the preflight job instead of implementing preflight logic inline. The reusable workflow reference SHALL be pinned to a specific commit SHA.

#### Scenario: Preflight job calls reusable workflow
- **WHEN** the release workflow is triggered via `workflow_dispatch` with a `tag` input
- **THEN** the `preflight` job SHALL use `complytime/org-infra/.github/workflows/reusable_release_preflight.yml@<pinned-sha>`
- **AND** the `tag` input SHALL be forwarded to the reusable workflow

#### Scenario: CI checks are explicitly configured
- **WHEN** the preflight reusable workflow is called
- **THEN** the `ci_checks` input SHALL be set to `["Build and test", "Standardized CI / Run linters"]`
- **AND** auto-discovery SHALL NOT be relied upon

### Requirement: Release workflow uses org-infra reusable GoReleaser
The `release.yml` workflow SHALL invoke `complytime/org-infra/.github/workflows/reusable_release_goreleaser.yml` as the release job instead of implementing GoReleaser setup and execution inline. The reusable workflow reference SHALL be pinned to the same commit SHA as the preflight workflow.

#### Scenario: Release job calls reusable workflow
- **WHEN** the preflight job completes successfully and outputs a validated tag
- **THEN** the `release` job SHALL use `complytime/org-infra/.github/workflows/reusable_release_goreleaser.yml@<pinned-sha>`
- **AND** the `tag` input SHALL be set to `${{ needs.preflight.outputs.tag }}`

#### Scenario: Release job only runs after successful preflight
- **WHEN** the preflight job outputs an empty tag (validation failure)
- **THEN** the `release` job SHALL NOT execute

### Requirement: Workflow exposes skip override inputs
The rewritten `release.yml` SHALL expose `skip_semver_check`, `skip_ci_checks`, and `skip_unreleased_check` as `workflow_dispatch` boolean inputs (default `false`) and SHALL forward them to the reusable preflight workflow.

#### Scenario: Skip inputs are forwarded to preflight
- **WHEN** a release is triggered with `skip_semver_check: true`
- **THEN** the preflight reusable workflow SHALL receive `skip_semver_check: true`
- **AND** semver ordering verification SHALL be skipped

#### Scenario: Skip inputs default to false
- **WHEN** a release is triggered without specifying skip inputs
- **THEN** all skip inputs SHALL default to `false`
- **AND** all preflight checks SHALL execute

### Requirement: Workflow preserves existing trigger and permissions
The rewritten `release.yml` SHALL preserve the existing `workflow_dispatch` trigger with `tag` string input, and SHALL declare minimum required permissions for each job using least-privilege.

#### Scenario: Permissions are correctly scoped
- **WHEN** the release workflow is defined
- **THEN** the top-level `permissions` SHALL explicitly deny `contents: none` and `id-token: none`
- **AND** the preflight job SHALL have `contents: write` and `checks: read` permissions
- **AND** the release job SHALL have `contents: write` and `id-token: write` permissions

### Requirement: Reusable workflow references are SHA-pinned
All references to `complytime/org-infra` reusable workflows SHALL use a full commit SHA rather than a mutable tag or branch reference.

#### Scenario: SHA pin matches a known release
- **WHEN** the release workflow references org-infra reusable workflows
- **THEN** each `uses:` line SHALL reference a full 40-character commit SHA
- **AND** a comment SHALL indicate the corresponding version tag (e.g., `# v0.7.1`)
