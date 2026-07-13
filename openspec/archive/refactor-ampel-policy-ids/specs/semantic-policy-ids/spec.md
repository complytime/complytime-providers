## ADDED Requirements

### Requirement: Testdata policies use semantic IDs
All granular policy files in `cmd/ampel-provider/convert/testdata/policies/` SHALL use semantic, benchmark-agnostic `id` fields matching the canonical IDs defined in org-infra.

#### Scenario: Policy files have semantic IDs
- **GIVEN** the testdata policy files have been created with new semantic IDs
- **WHEN** the testdata policies directory is loaded by `LoadGranularPolicies`
- **THEN** the resulting map keys are: `require-pull-request`, `minimum-approvals`, `block-force-push`, `prevent-admin-bypass`, `require-code-owner-review`

#### Scenario: Policy filenames match IDs
- **GIVEN** the testdata policy files have been renamed
- **WHEN** the testdata policies directory is listed
- **THEN** each filename is `<semantic-id>.json` (e.g., `require-pull-request.json`)

### Requirement: Testdata control references use semantic IDs
Each testdata policy's `meta.controls[]` entry referencing the `repo-branch-protection` framework SHALL use a semantic control ID matching the canonical IDs defined in org-infra.

#### Scenario: Control references are updated
- **GIVEN** the testdata policy files have been updated with new control IDs
- **WHEN** any testdata policy file's `meta.controls` array is inspected
- **THEN** the entry with `"framework": "repo-branch-protection"` uses one of: `pull-request-enforcement`, `approval-requirements`, `force-push-restriction`, `admin-bypass-prevention`, `code-owner-enforcement`

#### Scenario: OSPS references are unchanged
- **GIVEN** the testdata policy files have been updated
- **WHEN** any testdata policy file has an OSPS framework control reference
- **THEN** the OSPS `class` and `id` values remain unchanged

### Requirement: Assessment plan fixtures use semantic IDs
Test fixtures for assessment plans SHALL use the new semantic requirement IDs.

#### Scenario: Full assessment plan references all semantic IDs
- **GIVEN** the full assessment plan fixture has been updated
- **WHEN** the full assessment plan fixture is loaded
- **THEN** it contains requirement IDs: `require-pull-request`, `minimum-approvals`, `block-force-push`, `prevent-admin-bypass`, `require-code-owner-review`

#### Scenario: Subset assessment plan references correct semantic IDs
- **GIVEN** the subset assessment plan fixture has been updated
- **WHEN** the subset assessment plan fixture is loaded
- **THEN** it contains requirement IDs: `require-pull-request` and `block-force-push`

### Requirement: Expected bundle fixtures are consistent
Expected bundle fixtures SHALL be structurally equivalent to the output produced by `LoadGranularPolicies` + `MatchPolicies` + `MergeToBundle` using the current testdata source policies. Policy IDs, control references, CEL expressions, predicate URLs, and messages MUST match.

#### Scenario: Full expected bundle matches source policies
- **GIVEN** the testdata source policies and full assessment plan use semantic IDs
- **WHEN** the full expected bundle is compared against the bundle generated from testdata policies using the full assessment plan
- **THEN** they are structurally equivalent (same IDs, controls, tenets, and messages)

#### Scenario: Subset expected bundle matches source policies
- **GIVEN** the testdata source policies and subset assessment plan use semantic IDs
- **WHEN** the subset expected bundle is compared against the bundle generated from testdata policies using the subset assessment plan
- **THEN** they are structurally equivalent, using the current CEL expressions and dual predicate URLs from the source policies

### Requirement: All tests pass after ID updates
All unit tests in the `convert` package SHALL pass after updating both fixture files and hardcoded ID references in the test Go code. Test logic and structure SHALL remain unchanged; only ID string literals are updated.

#### Scenario: Test suite passes
- **GIVEN** all testdata fixtures and `convert_test.go` assertion strings have been updated to use semantic IDs
- **WHEN** `go test ./cmd/ampel-provider/convert/...` is run
- **THEN** all tests pass
