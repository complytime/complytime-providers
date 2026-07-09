## ADDED Requirements

### Requirement: Operational errors use ScanResponse.Errors

When the OPA provider encounters an operational error during scanning (failed
git clone, bundle-pull failure, tool invocation failure) that produces no
findings for a target, the provider SHALL report the error via
`ScanResponse.Errors` instead of creating a synthetic assessment entry with
`RequirementID = "scan-error"`.

#### Scenario: OPA target fails with no findings

- **WHEN** an OPA target has `Status == "error"` and zero findings
- **THEN** `ToScanResponse` SHALL append the error to `resp.Errors`
- **THEN** `resp.Assessments` SHALL NOT contain an entry with `RequirementID == "scan-error"` for that target

#### Scenario: Target fails but has partial findings

- **WHEN** a target has `Status == "error"` but also has one or more findings
- **THEN** the findings SHALL still be mapped to their respective `AssessmentLog` entries with `Result == ResultError`
- **THEN** the operational error SHALL NOT be duplicated into `resp.Errors` (findings already carry the error status via `mapResult`)

### Requirement: Write errors use ScanResponse.Errors

When the OPA provider encounters a write error while persisting scan results
to disk, the error SHALL be reported via `ScanResponse.Errors` instead of
being embedded as a `"result-persistence"` step in the `"scan-status"`
assessment.

#### Scenario: Write error during OPA scan

- **WHEN** `server.Scan()` encounters a write error
- **THEN** the write error SHALL be appended to `resp.Errors`
- **THEN** `ScanStatusAssessment` SHALL NOT contain a `"result-persistence"` step

#### Scenario: No write error during OPA scan

- **WHEN** `server.Scan()` completes without write errors
- **THEN** `resp.Errors` SHALL NOT contain any write-error entries
- **THEN** `ScanStatusAssessment` SHALL report only target scan outcomes

### Requirement: No scan-error sentinel remains

The `const errorReqID = "scan-error"` declaration and all associated grouping
logic SHALL be removed from the OPA provider's `ToScanResponse` function.

#### Scenario: Codebase contains no scan-error sentinel in OPA provider

- **WHEN** the migration is complete
- **THEN** no Go source file in `cmd/opa-provider/results/` SHALL contain the string `"scan-error"`

### Requirement: ScanStatusAssessment signature update

The `ScanStatusAssessment` function SHALL no longer accept a `writeErr`
parameter. Its signature SHALL be
`func ScanStatusAssessment(targetResults []*PerTargetResult) provider.AssessmentLog`.

#### Scenario: ScanStatusAssessment without write error parameter

- **WHEN** `ScanStatusAssessment` is called
- **THEN** it SHALL return an `AssessmentLog` with `RequirementID == "scan-status"` summarizing target scan outcomes
- **THEN** it SHALL NOT include any `"result-persistence"` step

### Requirement: TODO comment removal

The TODO comment at `cmd/opa-provider/results/results.go` referencing
complyctl#510 SHALL be removed since the upstream change has merged.

#### Scenario: TODO comment absent

- **WHEN** the migration is complete
- **THEN** the file `cmd/opa-provider/results/results.go` SHALL NOT contain a TODO comment referencing complyctl#510
