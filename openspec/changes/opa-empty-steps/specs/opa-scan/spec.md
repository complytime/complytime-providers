## MODIFIED Requirements

### Requirement: ToScanResponse produces assessments for all mapped requirements
`ToScanResponse` MUST produce an `AssessmentLog` entry with at least one
`Step` for every requirement ID present in the `reverseMap`, regardless of
whether findings (failures/warnings) exist for that requirement. Requirements
with no findings MUST receive a passing assessment with one step per
successfully-scanned target.

Previously: `ToScanResponse` only created `AssessmentLog` entries for
requirements that had at least one finding. Requirements where all checks
passed produced no assessment, resulting in `steps: []` in the evaluation
log and failing Gemara CUE schema validation.

#### Scenario: All checks pass for a mapped requirement
- **GIVEN** a `reverseMap` containing Rego namespace to Gemara ID mappings
- **WHEN** conftest evaluates policies and produces zero findings for a
  mapped requirement
- **THEN** `ToScanResponse` MUST return an `AssessmentLog` for that
  requirement with one `Step` per scanned target, each with
  `Result == ResultPassed` and a non-empty message

#### Scenario: Mixed findings and passing requirements
- **GIVEN** a `reverseMap` with multiple requirement mappings
- **WHEN** some requirements have findings (failures/warnings) and others
  have none
- **THEN** `ToScanResponse` MUST return `AssessmentLog` entries for both:
  failing assessments with violation steps, and passing assessments with
  passing steps

#### Scenario: Error targets excluded from passing assessments
- **GIVEN** targets with mixed statuses (`"scanned"` and `"error"`)
- **WHEN** `ToScanResponse` synthesizes passing assessments for requirements
  with no findings
- **THEN** only targets with `Status == "scanned"` MUST contribute steps
  to the passing assessments; error targets MUST NOT appear in synthesized
  passing steps

#### Scenario: No reverse map provided
- **GIVEN** `reverseMap` is nil
- **WHEN** `ToScanResponse` processes target results
- **THEN** no passing assessments MUST be synthesized (backward-compatible
  behavior)
