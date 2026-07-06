## ADDED Requirements

### Requirement: Passing assessments for requirements with no findings

The `ToScanResponse` function SHALL produce an `AssessmentLog` entry for every requirement ID in the provided requirement ID list, including requirements that had zero findings during the scan.

#### Scenario: Requirement with zero findings gets passing assessment

- **WHEN** a requirement ID appears in the all-requirement-IDs list but has no findings in the scan results
- **THEN** an `AssessmentLog` SHALL be created for that requirement with at least one `Step` having `Result` set to `ResultPassed`

#### Scenario: Requirement with findings retains existing behavior

- **WHEN** a requirement ID appears in findings
- **THEN** the `AssessmentLog` SHALL be constructed from the findings as before, regardless of whether the requirement ID also appears in the all-requirement-IDs list

#### Scenario: No requirement ID list provided (nil)

- **WHEN** `ToScanResponse` is called with a nil requirement ID list
- **THEN** the function SHALL produce assessments only from findings (backward-compatible behavior)

### Requirement: Synthetic steps reflect scan targets

Synthetic passing steps for requirements with no findings SHALL identify the scan targets (repositories and branches) that were evaluated.

#### Scenario: Synthetic step naming

- **WHEN** a synthetic passing step is created for a repository result
- **THEN** the step name SHALL use the format `<repoDisplayName>@<branch>`

#### Scenario: Error repositories excluded from synthetic steps

- **WHEN** a repository result has an error status
- **THEN** that repository SHALL NOT produce a synthetic passing step

### Requirement: Assessment message for passing requirements

Passing assessments synthesized for requirements with zero findings SHALL include a summary message indicating all repositories passed.

#### Scenario: All repositories passed message

- **WHEN** a synthetic passing assessment is created with N non-error repositories
- **THEN** the assessment message SHALL indicate that N of N repositories passed
