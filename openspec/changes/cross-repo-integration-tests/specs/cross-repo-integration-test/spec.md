## ADDED Requirements

### Requirement: complytime-providers CI triggers cross-repo integration on PRs

The complytime-providers repository SHALL have a GitHub Actions workflow at
`.github/workflows/ci_cross_repo_integration.yml` that triggers on pull requests
targeting `main` (opened and synchronize events) and on push to `main`. The workflow
SHALL build the Ampel provider from the current branch, check out `complyctl@main`,
build complyctl and the mock OCI registry, install `snappy` and `ampel`, and run
`tests/cross-repo/cross_repo_integration_test.sh` from the complyctl checkout.

#### Scenario: Workflow triggers on PR to main

- **WHEN** a pull request targeting `main` is opened or synchronized in
  complytime-providers
- **THEN** the `ci_cross_repo_integration` workflow is triggered and runs the full
  integration test

#### Scenario: Ampel provider binary comes from the PR branch

- **WHEN** the workflow runs
- **THEN** `complyctl-provider-ampel` is built from the PR branch source, not from a
  release artifact or from main

#### Scenario: complyctl binary and test script come from complyctl main

- **WHEN** the workflow runs
- **THEN** the `complyctl` binary, `mock-oci-registry` binary, and
  `tests/cross-repo/cross_repo_integration_test.sh` are sourced from `complyctl@main`
  checked out into `_complyctl/`

#### Scenario: Test script is not duplicated in complytime-providers

- **WHEN** the workflow runs
- **THEN** no copy of `cross_repo_integration_test.sh` exists in the
  complytime-providers repository; the script is always executed from the complyctl
  checkout

#### Scenario: Ampel provider build failure stops the workflow

- **WHEN** the Ampel provider build step (`make build`) fails
- **THEN** the workflow exits with a non-zero status before the test script runs, and
  the GitHub Actions job reports failure with the build error output visible in the
  step log

#### Scenario: Workflow uses minimum required permissions

- **WHEN** the workflow is defined
- **THEN** it declares `contents: read` at the workflow level and no additional
  permissions beyond what is required to check out repositories and read secrets

#### Scenario: Workflow uses consistent Go version and runner

- **WHEN** the workflow is defined
- **THEN** the Go version and runner image match the existing CI workflow
  (`ci_local.yml`) to ensure consistent build behavior across all CI jobs
