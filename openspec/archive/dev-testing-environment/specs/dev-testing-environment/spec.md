## ADDED Requirements

This spec mirrors complyctl's `dev-testing-environment` spec with the
build order reversed: providers from local source, complyctl from main.
Shared requirements (Containerfile packages, tool installations, mock
registry, GITHUB_TOKEN handling, documentation patterns) follow
complyctl's spec (FR-001 through FR-003). Providers-specific
differences and inline enumerations of applicable shared scenarios
are detailed below.

### FR-001: Devcontainer configuration provides a Fedora testing environment

The `.devcontainer/` configuration SHALL mirror complyctl's Containerfile
and devcontainer.json (same base image, system packages, GOTOOLCHAIN,
SELinux runArgs, hostRequirements of 4 cpus and 16gb memory). See
complyctl spec FR-001.

#### Scenario: Devcontainer configuration is present and valid

- **GIVEN** the repository contains `.devcontainer/devcontainer.json`
  and `.devcontainer/Containerfile`
- **WHEN** a maintainer opens the repository in a devcontainer-compatible
  tool (GitHub Codespaces, DevPod, or VS Code Dev Containers)
- **THEN** the tool SHALL detect `.devcontainer/devcontainer.json` and
  build the environment from the Containerfile

### FR-002: Post-create setup builds providers from local source and complyctl from main

The post-create script SHALL mirror complyctl's operational patterns
(strict error handling via `set -euo pipefail`, pinned tool versions,
GITHUB_TOKEN least-privilege, mock registry with nohup/disown,
auto-rebuild hook, private bundle registration) with the build order
reversed. The script SHALL NOT pass GITHUB_TOKEN to subprocesses that
do not require it: capture the token into a local variable, unset it
from the environment, and only pass it to processes that need it. The
token SHALL NOT appear in logs, script output, or error messages.

#### Scenario: Provider binaries are built from local source

- **GIVEN** the devcontainer has been built from the Containerfile
- **WHEN** the devcontainer post-create command completes
- **THEN** provider binaries SHALL be built from the local repository
  source via `make build`. Known providers (`complyctl-provider-openscap`,
  `complyctl-provider-ampel`, `complyctl-provider-opa`) SHALL be copied
  to `~/.complytime/providers/` if present, with a WARNING for any
  missing binaries (the OPA provider is not yet in this repository)

#### Scenario: complyctl and mock-oci-registry are built from main

- **GIVEN** the devcontainer has been built from the Containerfile
- **WHEN** the devcontainer post-create command completes
- **THEN** `complyctl` and `mock-oci-registry` SHALL be built from the
  latest complyctl main branch and available on PATH. The cloned commit
  SHA SHALL be logged for auditability

#### Scenario: complyctl clone fails gracefully

- **GIVEN** the devcontainer has been built from the Containerfile
- **WHEN** the git clone of complyctl from main fails (network error,
  authentication failure, rate limiting)
- **THEN** the post-create script SHALL exit with a non-zero status and
  a clear error message identifying the clone URL and the failure,
  suggesting GITHUB_TOKEN for rate limit issues

#### Scenario: External tools are installed at pinned versions

- **GIVEN** the devcontainer has been built from the Containerfile
- **WHEN** the devcontainer post-create command completes
- **THEN** `snappy` (`github.com/carabiner-dev/snappy@v0.2.4`), `ampel`
  (`github.com/carabiner-dev/ampel/cmd/ampel@v1.2.1`), and `conftest`
  (`github.com/open-policy-agent/conftest@v0.68.2`) SHALL be available
  on PATH, installed via `go install` at pinned version tags

#### Scenario: Test workspace is configured with Gemara content

- **GIVEN** the post-create command has completed successfully
- **WHEN** a maintainer inspects the test workspace
- **THEN** a `complytime.yaml` workspace configuration SHALL exist in
  `~/test-workspace/` pointing to the mock OCI registry on localhost,
  and the mock registry SHALL be running on port 8765 with output
  logged to `/tmp/mock-oci-registry.log`

#### Scenario: Post-create fails clearly on setup errors

- **GIVEN** a setup step fails (e.g., `make build`, `go install`,
  `git clone`)
- **WHEN** the post-create script encounters the failure
- **THEN** the script SHALL exit with a non-zero status and display
  an error message identifying the failed step

#### Scenario: Post-create succeeds without GITHUB_TOKEN

- **GIVEN** the environment does NOT have `GITHUB_TOKEN` set
- **WHEN** the devcontainer post-create command runs
- **THEN** the script SHALL complete successfully (exit code 0) and
  emit a warning that `GITHUB_TOKEN` is required for `complyctl scan`

#### Scenario: Auto-rebuild on provider source change

- **GIVEN** the post-create command has completed successfully and the
  mock registry is running on port 8765
- **WHEN** the user checks out a different branch and opens a new shell
- **THEN** the environment SHALL detect the source change (different
  HEAD commit) and automatically rebuild provider binaries. The user
  SHALL be able to skip auto-rebuild by setting
  `COMPLYCTL_SKIP_REBUILD=1` (any non-empty value skips the rebuild)

#### Scenario: complyctl commands work end-to-end with PR providers

- **GIVEN** the post-create command has completed successfully and the
  mock registry is running on port 8765
- **WHEN** a maintainer runs `complyctl get` followed by
  `complyctl generate --policy-id test-ampel-bp` in the test workspace
- **THEN** both commands SHALL complete successfully using the provider
  binaries built from the local PR branch

#### Scenario: complyctl scan works when GITHUB_TOKEN is configured

- **GIVEN** the environment has `GITHUB_TOKEN` set with read access to
  public repositories
- **WHEN** a maintainer runs
  `complyctl scan --policy-id test-ampel-bp` in the test workspace
- **THEN** the command SHALL complete successfully and produce scan
  results

#### Scenario: complyctl scan fails gracefully without GITHUB_TOKEN

- **GIVEN** the environment does NOT have `GITHUB_TOKEN` set
- **WHEN** a maintainer runs
  `complyctl scan --policy-id test-ampel-bp` in the test workspace
- **THEN** the command SHALL exit with a non-zero code and output a
  message indicating the token is required

### FR-003: CI validates Containerfile builds

The Makefile SHALL include a `test-devcontainer` target that builds
the Containerfile using `podman build .devcontainer/` to verify the
image definition is valid. The post-create script SHALL pass
`shellcheck` linting without errors. Post-create script coverage is
integration-level via manual verification (Section 6 of tasks.md).
Unit testing of individual script functions is not required due to
the imperative, environment-dependent nature of the setup logic.

#### Scenario: Containerfile builds successfully

- **GIVEN** the `.devcontainer/Containerfile` exists
- **WHEN** a maintainer or CI runs `make test-devcontainer`
- **THEN** the command SHALL exit with code 0

#### Scenario: Post-create script passes shellcheck

- **GIVEN** the `.devcontainer/scripts/post-create.sh` exists
- **WHEN** `shellcheck` is run against the script
- **THEN** the linter SHALL report no errors

### FR-004: Documentation explains provider testing workflows

The repository SHALL include documentation in `docs/` that covers
provider-specific setup and references complyctl's
`docs/TESTING_ENVIRONMENT.md` (via GitHub `main` branch blob URL) for
shared concepts (Codespaces, DevPod, VS Code, GITHUB_TOKEN,
troubleshooting). See complyctl spec FR-003 for shared documentation
requirements.

#### Scenario: Documentation is discoverable from README

- **WHEN** a maintainer reads the repository README
- **THEN** they SHALL find a link to the dev testing environment
  documentation

#### Scenario: Documentation references complyctl docs

- **WHEN** a maintainer reads the dev testing environment documentation
- **THEN** they SHALL find references to complyctl's documentation for
  shared setup and troubleshooting details
