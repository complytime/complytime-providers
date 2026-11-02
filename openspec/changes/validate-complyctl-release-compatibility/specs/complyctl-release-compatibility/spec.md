# Spec Delta

## Purpose

Validates that provider binaries interoperate with the supported stable
`complyctl` release before a provider dependency update can merge.

## ADDED Requirements

### Requirement: Stable release baseline selection
The CI workflow MUST select the highest stable `complyctl` release tag in the
provider module's declared compatibility line: the same major version for a
v1+ dependency or the same minor version for a v0 dependency. Prerelease and
malformed tags MUST be excluded. The workflow MUST report the selected tag and
resolved commit, check out and execute that resolved commit, and MUST fail when
it cannot resolve or verify a valid baseline.

#### Scenario: Stable release is available
- **WHEN** stable release tags exist for the declared dependency compatibility line
- **THEN** CI selects the highest stable tag and reports its tag and commit

#### Scenario: No stable release is available
- **WHEN** CI cannot resolve a stable release for the declared dependency major
  version
- **THEN** CI fails with a release-baseline diagnostic before integration runs

#### Scenario: Resolved commit cannot be verified
- **WHEN** the released checkout does not resolve to the selected commit
- **THEN** CI fails with a release-baseline diagnostic before build or integration runs

### Requirement: Released complyctl integration validation
The CI workflow MUST build all provider binaries from an isolated workspace
whose `complyctl` module dependency is temporarily replaced with the selected
stable release checkout, and run the existing Ampel-focused cross-repository
integration validation against that checkout. A successful result MUST require
both the provider build and the integration validation to succeed.

#### Scenario: Providers support the selected release
- **WHEN** all provider binaries build and the existing Ampel integration harness
  interoperates with the selected release
- **THEN** the release compatibility validation succeeds

#### Scenario: Provider API usage is incompatible
- **WHEN** providers cannot build or complete integration against the selected
  release
- **THEN** the release compatibility validation fails

### Requirement: Actionable compatibility diagnostics
The release compatibility validation MUST identify whether failure occurred
while resolving the release baseline, building providers, preparing the
released `complyctl` checkout, or running integration. It MUST preserve the
existing integration result for diagnosis.

#### Scenario: Released checkout lacks integration prerequisites
- **WHEN** the selected release cannot provide the prerequisites for the
  cross-repository integration validation
- **THEN** CI fails with a diagnostic that identifies the selected release and
  the missing prerequisite

#### Scenario: Integration command fails
- **WHEN** the cross-repository integration validation returns a nonzero status
- **THEN** CI reports the integration failure without classifying it as a
  release-resolution or provider-build failure
