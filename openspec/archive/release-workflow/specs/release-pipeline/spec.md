## ADDED Requirements

### Requirement: Manual release trigger

The release workflow SHALL be triggered exclusively via `workflow_dispatch`
with a required `tag` input parameter (e.g., `v0.1.0`). The workflow MUST
NOT trigger automatically on tag push or any other event.

#### Scenario: Maintainer dispatches release

- **WHEN** a maintainer runs `gh workflow run release.yml -f tag=v0.1.0`
- **THEN** the release workflow starts with the preflight job

#### Scenario: Tag push does not trigger release

- **WHEN** a tag matching `v*` is pushed to the repository
- **THEN** the release workflow MUST NOT execute

### Requirement: Tag format validation

The preflight job SHALL reject tags that do not match the strict semver
format `vMAJOR.MINOR.PATCH` (e.g., `v0.1.0`, `v1.2.3`).

#### Scenario: Valid tag format

- **WHEN** the tag input is `v0.1.0`
- **THEN** the preflight job proceeds to the next validation step

#### Scenario: Invalid tag format rejected

- **WHEN** the tag input is `0.1.0` or `v1.0` or `v1.0.0-rc1`
- **THEN** the preflight job fails with an error message describing the
  expected format

### Requirement: Tag uniqueness

The preflight job SHALL verify that the requested tag does not already
exist as a remote tag pointing to a different commit than HEAD.

#### Scenario: Tag does not exist yet

- **WHEN** the tag `v0.1.0` does not exist in the remote repository
- **THEN** the preflight job proceeds to the next validation step

#### Scenario: Tag already exists pointing to a different commit

- **WHEN** the tag `v0.1.0` already exists in the remote repository and
  points to a commit other than HEAD
- **THEN** the preflight job fails with an error stating the tag exists
  at a different commit

#### Scenario: Tag exists pointing to HEAD (re-run after partial failure)

- **WHEN** the tag `v0.1.0` already exists in the remote repository and
  points to the same commit as HEAD
- **THEN** the preflight job proceeds (enables safe re-runs when the
  release job failed after tag creation)

### Requirement: Semver ordering

The preflight job SHALL verify that the requested tag is greater than
the latest existing release tag when sorted by semver rules. This check
is skipped when the requested tag already exists at HEAD (re-run).

#### Scenario: First release

- **WHEN** no existing release tags are found
- **THEN** the preflight job proceeds (any valid tag is accepted)

#### Scenario: Version is greater than latest

- **WHEN** the latest existing tag is `v0.1.0` and the requested tag is
  `v0.2.0`
- **THEN** the preflight job proceeds

#### Scenario: Version is not greater than latest

- **WHEN** the latest existing tag is `v0.2.0` and the requested tag is
  `v0.1.0`
- **THEN** the preflight job fails with an error stating the ordering
  violation

#### Scenario: Re-run -- requested tag is latest and points to HEAD

- **WHEN** the latest existing tag is `v0.1.0`, the requested tag is
  `v0.1.0`, and the tag points to HEAD
- **THEN** the semver ordering check is skipped (re-run after partial
  failure)

### Requirement: CI verification

The preflight job SHALL verify that all required CI checks have passed
on the HEAD commit of the default branch by querying the GitHub Checks API.
The list of required check names SHALL be defined as constants in the
workflow with comments documenting their source workflow files.

#### Scenario: All required checks passed

- **WHEN** the checks `Build and test` (from `ci_local.yml`) and
  `Standardized CI / Run linters` (from `ci_checks.yml` calling
  `reusable_ci.yml`) both report `conclusion: success` for HEAD
- **THEN** the preflight job proceeds

#### Scenario: Required check has not passed

- **WHEN** any required check reports a conclusion other than `success`
  or has not run
- **THEN** the preflight job fails with an error identifying the failing
  check name and its status

### Requirement: Unreleased commits guard

The preflight job SHALL verify that there are unreleased commits since
the latest tag (or since the initial commit if no tags exist). This
check is skipped when the requested tag already exists at HEAD (re-run).

#### Scenario: Commits exist since last tag

- **WHEN** there are 5 commits between the latest tag and HEAD
- **THEN** the preflight job proceeds and logs the commit count

#### Scenario: No commits since last tag

- **WHEN** HEAD is the same commit as the latest tag and the requested
  tag does not already exist
- **THEN** the preflight job fails with an error stating nothing to release

#### Scenario: Re-run -- requested tag exists at HEAD

- **WHEN** the requested tag `v0.1.0` already exists at HEAD and is the
  latest tag
- **THEN** the unreleased commits check is skipped (re-run after partial
  failure)

### Requirement: Annotated tag creation

The preflight job SHALL create an annotated git tag and push it to the
remote repository after all validation steps pass.

#### Scenario: Tag created successfully

- **WHEN** all preflight validations pass and the tag does not exist
- **THEN** the workflow creates an annotated tag at HEAD and pushes it

#### Scenario: Tag already exists on re-run

- **WHEN** the preflight is re-run after a partial failure and the tag
  was already created pointing to HEAD
- **THEN** the tag creation step skips gracefully without error

### Requirement: Concurrency protection

The release workflow SHALL define a concurrency group that prevents
parallel release executions. The group MUST NOT cancel in-progress runs.

#### Scenario: Concurrent release attempts

- **WHEN** two `workflow_dispatch` events are triggered in quick succession
- **THEN** the second run waits for the first to complete (does not cancel)

### Requirement: Workflow permissions

The release workflow SHALL declare `permissions: {}` at the workflow level
and explicit least-privilege per-job permissions. No permissions SHALL be
granted beyond what each job requires.

#### Scenario: Preflight job permissions

- **WHEN** the preflight job executes
- **THEN** it operates with `contents: write` (to push the annotated tag)
  and `checks: read` (to query CI status via the Checks API)

#### Scenario: Release job permissions

- **WHEN** the release job executes
- **THEN** it operates with `contents: write` (to create the GitHub
  Release and upload assets) and `id-token: write` (for Sigstore keyless
  signing via OIDC)

### Requirement: Pinned tool versions

GitHub Actions used in the release workflow SHALL be pinned to specific
commit SHAs with version comments. GoReleaser, cosign, and syft installer
actions SHALL use specific release versions, not `latest` or floating
tags. This is consistent with the existing CI workflows in this repository.

#### Scenario: Actions pinned by SHA

- **WHEN** the release workflow is inspected
- **THEN** all `uses:` references specify a commit SHA with a version
  comment (e.g., `actions/checkout@<sha> # v6.0.3`)

### Requirement: Per-provider binary archives

The release job SHALL produce one tar.gz archive per provider binary,
each containing a single executable.

#### Scenario: Release artifacts produced

- **WHEN** the release job completes successfully
- **THEN** the GitHub Release contains three archives:
  `complyctl-provider-openscap_linux_x86_64.tar.gz`,
  `complyctl-provider-ampel_linux_x86_64.tar.gz`, and
  `complyctl-provider-opa_linux_x86_64.tar.gz`

### Requirement: Supply chain artifacts

The release job SHALL produce checksums, cosign keyless signatures, and
SBOMs for all release artifacts. Cosign SHALL use Sigstore keyless
signing with the GitHub Actions OIDC issuer.

#### Scenario: Checksums and signatures

- **WHEN** the release job completes successfully
- **THEN** the GitHub Release contains `checksums.txt` and
  `checksums.txt.sigstore.json` (cosign keyless signature bundle using
  the GitHub Actions OIDC issuer
  `https://token.actions.githubusercontent.com`)

#### Scenario: Cosign signing failure

- **WHEN** cosign keyless signing fails (e.g., Sigstore infrastructure
  unavailable or OIDC token acquisition failure)
- **THEN** the release job fails and no GitHub Release is published

#### Scenario: SBOMs generated

- **WHEN** the release job completes successfully
- **THEN** the GitHub Release contains 4 SPDX JSON SBOMs: one for each
  of the three provider archives (named
  `<archive-name>.tar.gz.sbom.json`) and one for the source (named
  `source.tar.gz.sbom.json`)

### Requirement: Auto-generated changelog

The release job SHALL produce a changelog from conventional commits,
grouped by type (features, bug fixes, security, dependencies,
infrastructure, documentation). The changelog filter patterns are
defined in the GoReleaser configuration.

#### Scenario: Changelog in GitHub Release

- **WHEN** the release job completes successfully
- **THEN** the GitHub Release body contains a changelog grouped by
  conventional commit type, excluding `docs:`, `test:`, and merge
  pull request commits

### Requirement: OPA provider in RPM spec

The RPM spec SHALL include an OPA provider sub-package alongside
openscap and ampel.

#### Scenario: OPA sub-package built

- **WHEN** the RPM is built from the spec
- **THEN** a `complytime-providers-opa` sub-package is produced containing
  the `complyctl-provider-opa` binary at
  `/usr/libexec/complytime/providers/complyctl-provider-opa`

### Requirement: Packit target alignment

The Packit configuration SHALL target the same active Fedora versions as
complyctl across all job sections (copr_build, tests, propose_downstream,
koji_build, bodhi_update). CentOS Stream targets remain unchanged.

#### Scenario: Aligned Fedora targets in all job sections

- **WHEN** the `.packit.yaml` configuration is inspected
- **THEN** all job sections that previously referenced `fedora-42` or
  `f42` reference `fedora-44` or `f44` instead, while `centos-stream-9`
  and `centos-stream-10` targets remain unchanged

### Requirement: TMT test coverage for OPA

The TMT/FMF test plan SHALL validate the OPA provider binary alongside
openscap and ampel.

#### Scenario: OPA binary validated

- **WHEN** the TMT test plan executes after RPM installation
- **THEN** the test verifies that `complyctl-provider-opa` is executable
  at `/usr/libexec/complytime/providers/complyctl-provider-opa`
