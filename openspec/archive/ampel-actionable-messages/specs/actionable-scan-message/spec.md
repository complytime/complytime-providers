## ADDED Requirements

### Requirement: Message uses tenet failure text on non-passing results

When any step in a requirement group has a non-passing result, the
`AssessmentLog.Message` SHALL use the first non-passing step's
`Message` (the tenet `error.message`) instead of the generic count
string. This aligns with the OPA provider's existing behavior and
surfaces the actionable policy-defined failure reason.

#### Scenario: Single repository fails a tenet check

- **WHEN** a single `PerRepoResult` contains a finding with
  `Result: "fail"` and `Reason: "Pull/Merge requests can be merged
  without peer review."`
- **THEN** the `AssessmentLog.Message` for that requirement SHALL be
  `"Pull/Merge requests can be merged without peer review."` (the
  tenet's `error.message`)

#### Scenario: Multiple repositories with mixed results

- **WHEN** two `PerRepoResult` entries produce findings for the same
  requirement, one passing and one failing with
  `Reason: "Direct pushes are enabled so Pull/Merge requests are not
  required."`
- **THEN** the `AssessmentLog.Message` SHALL be `"Direct pushes are
  enabled so Pull/Merge requests are not required."` (the first
  non-passing step's message)

#### Scenario: All repositories pass a tenet check

- **WHEN** all `PerRepoResult` entries produce passing findings for
  a requirement
- **THEN** the `AssessmentLog.Message` SHALL use the first passing
  step's message (the tenet's `assessment.message`) rather than a
  generic count string

### Requirement: Vocabulary accuracy in fallback message

When the message override logic cannot find a non-empty step message,
the fallback count string SHALL use "checks" instead of "repositories"
to accurately describe what is being counted.

#### Scenario: Steps with empty messages fall back to count string

- **WHEN** all steps in a requirement group have empty `Message`
  fields
- **THEN** the `AssessmentLog.Message` SHALL be
  `"X of Y checks passed"` where X is the number of passing steps
  and Y is the total number of steps
