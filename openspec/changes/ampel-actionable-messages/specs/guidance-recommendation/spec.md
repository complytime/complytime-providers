## ADDED Requirements

### Requirement: Propagate tenet guidance to Recommendation field

The Ampel provider SHALL propagate the `error.guidance` field from
tenet evaluation results to the `AssessmentLog.Recommendation` field.
This provides platform-specific remediation instructions (e.g.,
GitHub/GitLab configuration steps) in complyctl reports.

#### Scenario: Failed tenet with guidance

- **WHEN** a tenet evaluation fails and the Ampel attestation
  contains `error.guidance: "GitHub: Set required_approving_review_count
  >= 1. GitLab: Set approvals_before_merge >= 1."`
- **THEN** the `Finding.Guidance` field SHALL contain the guidance
  text
- **AND** the `AssessmentLog.Recommendation` for the corresponding
  requirement SHALL be set to the first non-passing step's guidance

#### Scenario: Failed tenet without guidance

- **WHEN** a tenet evaluation fails and the Ampel attestation has no
  `error.guidance` field (or it is empty)
- **THEN** the `Finding.Guidance` field SHALL be empty
- **AND** the `AssessmentLog.Recommendation` SHALL be empty

#### Scenario: All tenets pass

- **WHEN** all tenet evaluations pass for a requirement
- **THEN** the `AssessmentLog.Recommendation` SHALL be empty (no
  remediation needed)

### Requirement: Finding struct carries guidance

The `Finding` struct SHALL include a `Guidance` field so that
`ParseAmpelOutput` can propagate `error.guidance` from the Ampel
attestation through to `ToScanResponse`.

#### Scenario: ParseAmpelOutput extracts guidance from failing tenet

- **WHEN** `ParseAmpelOutput` processes an `ampelEvalResult` with
  `status: "FAIL"` and `error.guidance: "Enable branch protection"`
- **THEN** the resulting `Finding` SHALL have
  `Guidance: "Enable branch protection"`

#### Scenario: ParseAmpelOutput handles passing tenet

- **WHEN** `ParseAmpelOutput` processes an `ampelEvalResult` with
  `status: "PASS"`
- **THEN** the resulting `Finding` SHALL have an empty `Guidance`
  field (assessments do not carry guidance)
