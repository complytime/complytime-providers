# Spec Delta

## Purpose

Verify current providers against the selected stable complyctl host
using reproducible, separately versioned canonical integration checks.

## ADDED Requirements

### Requirement: FR-001 Stable release baseline selection

CI MUST retain selection of the highest stable complyctl tag in the
direct dependency's compatibility line: the same major for v1+, or
minor for v0. It MUST exclude malformed and prerelease versions,
report the tag and resolved commit, and verify the released checkout
matches that commit. Resolution or verification failure MUST fail CI.

#### Scenario: Supported stable release is available
- **GIVEN** stable tags exist in the direct dependency's supported line
- **WHEN** CI resolves the released host
- **THEN** it selects the highest stable version in that line
- **AND** reports its tag and verified checkout commit

#### Scenario: Baseline cannot be resolved or verified
- **WHEN** no supported release resolves or checkout HEAD differs
- **THEN** CI fails before any integration command executes

### Requirement: FR-002 Independently versioned canonical harness

The released-baseline job MUST obtain its integration harness from a
reviewed full immutable complyctl commit independently of the released
host revision. It MUST verify the harness checkout and required host
selection interface. Missing, mismatched, or incompatible harnesses
MUST fail preparation without falling back to historical assertions,
mutable branches, or copied harness implementations.

#### Scenario: Harness revision differs from released host revision
- **GIVEN** the harness commit contains mapping-driven checks
- **WHEN** CI prepares the released-baseline job
- **THEN** it verifies that exact harness commit independently
- **AND** keeps the selected released host commit unchanged

#### Scenario: Harness preparation fails
- **WHEN** checkout verification or harness prerequisites fail
- **THEN** CI reports a harness preparation failure
- **AND** does not run a historical or main-branch fallback harness

### Requirement: FR-003 Released host remains the implementation tested

The job MUST build the selected released complyctl and mock registry
from their unchanged checkout and pass their absolute binary directory
through `COMPLYCTL_BIN_DIR`. Harness assertions
and fixtures MUST come from its own verified checkout. Preparation MUST
reject missing binaries or incompatible fixture support, not substitute
a main-branch host, registry, or patched released source.

#### Scenario: Released integration executes the selected binaries
- **GIVEN** both the harness and released checkout contain binaries
- **WHEN** the released-baseline integration executes
- **THEN** it invokes only the selected released host and registry
- **AND** uses the verified canonical harness's assertions and fixtures

#### Scenario: Selected released binary is missing
- **GIVEN** an alternate main-branch binary is available
- **WHEN** either selected released executable is unavailable
- **THEN** preparation fails without using the alternate binary

#### Scenario: Fixture contract is incompatible with the release
- **WHEN** the selected release cannot serve the canonical fixtures
- **THEN** CI fails with an actionable compatibility diagnostic
- **AND** does not rebuild the registry from the harness revision

### Requirement: FR-004 Released provider dependency validation

The job MUST preserve its isolated provider workspace, temporarily
replace its complyctl dependency with the selected released checkout,
and build and test all provider binaries there. Both provider build and
test success MUST precede integration. It MUST NOT change the committed
dependency or accept provider binaries built against another host as
the released-baseline result.

#### Scenario: Providers build and test against the selected release
- **WHEN** the isolated replacement, all-provider build, and tests pass
- **THEN** integration uses those providers with the released host

#### Scenario: Dependency resolution or provider checks fail
- **WHEN** replacement, dependency resolution, build, or tests fail
- **THEN** CI identifies the failing stage and release
- **AND** does not continue integration or reuse other providers

### Requirement: FR-005 Canonical assertions remain mandatory

Released-baseline success MUST require the canonical Ampel/OPA harness
to succeed, including mapped evidence integrity and existing pipeline
assertions. Nonzero integration results MUST remain failures. The job
MUST NOT skip assertions, restore moving aliases, repoint the host,
or relax timeouts and quality gates to accommodate retained evidence.

#### Scenario: Retained evidence interoperates with the released host
- **GIVEN** released-host provider builds and tests succeed
- **WHEN** the canonical harness verifies mapped retained attestations
- **THEN** released integration succeeds without a host release

#### Scenario: Evidence or another existing assertion fails
- **WHEN** the canonical integration command returns nonzero
- **THEN** the released job fails and preserves its diagnostics

### Requirement: FR-006 Reproducible and distinct diagnostics

CI MUST report the released host tag/commit and canonical harness
commit in logs and its job summary. Failures MUST distinguish baseline
resolution, host preparation/build, harness preparation, provider
dependency/build/test, and integration. It MUST preserve failure output
without exposing credentials and keep the separate main-consumer job.

#### Scenario: Maintainer reproduces a failed released-baseline run
- **WHEN** the job reports an integration failure
- **THEN** logs identify both exact revisions and the failing stage
- **AND** the canonical harness's non-secret diagnostics remain visible

#### Scenario: Main-consumer validation also runs
- **WHEN** provider CI validates main and the released baseline
- **THEN** the jobs retain distinct host provenance and results
- **AND** a passing main result cannot replace a failed released result

### Requirement: FR-007 Fail-closed execution regressions

Offline regression coverage MUST execute the integration wiring with
distinguishable host, registry, and harness fixtures. It MUST prove
explicit released-binary selection, missing-input rejection, and
nonzero result propagation. Text-only workflow assertions MUST NOT be
the sole proof of which binaries execute.

#### Scenario: Alternate binaries would hide wrong-host selection
- **GIVEN** default and released fixtures produce distinct traces
- **WHEN** the test executes released-baseline integration wiring
- **THEN** the trace proves the released host and registry executed
- **AND** neither default executable was invoked

#### Scenario: Required input or integration fails offline
- **WHEN** an input is missing, a revision mismatches, or harness fails
- **THEN** the regression verifies the failure remains nonzero
- **AND** verifies no fallback execution occurred
