## ADDED Requirements

### Requirement: OPA provider declares url and input_path as optional group
The OPA provider's `Describe()` SHALL declare `url` and `input_path` in
`OptionalTargetVariableGroups` as `"url|input_path"` instead of in
`RequiredTargetVariables`. This allows `complyctl doctor` to validate
that at least one of the two is present, rather than demanding both.

#### Scenario: Describe returns optional target variable groups
- **WHEN** the OPA provider's `Describe()` is called
- **THEN** the response SHALL contain `OptionalTargetVariableGroups` with
  `["url|input_path"]`
- **AND** `RequiredTargetVariables` SHALL NOT contain `url` or `input_path`
