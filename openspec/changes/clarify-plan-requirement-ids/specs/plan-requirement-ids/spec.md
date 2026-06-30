## MODIFIED Requirements

### Requirement: MappingEntry doc string accuracy

The `MappingEntry` struct doc string in `cmd/opa-provider/generate/mapping.go`
SHALL accurately describe the `RequirementID` field as holding a Gemara
`requirement-id` value, not a "Gemara assessment plan RequirementID."

#### Scenario: Doc string reflects correct semantics

- **WHEN** the `MappingEntry` struct is documented
- **THEN** the doc string SHALL state that `RequirementID` maps a Gemara
  requirement ID (from `AssessmentPlan.RequirementId`) to a Rego namespace
- **THEN** the doc string SHALL NOT conflate plan IDs with requirement IDs

### Requirement: MatchRequirements doc string accuracy

The `MatchRequirements` function doc string in
`cmd/opa-provider/generate/mapping.go` SHALL accurately describe that it
matches assessment configuration requirement IDs against mapping entries.

#### Scenario: Doc string reflects correct field usage

- **WHEN** the `MatchRequirements` function is documented
- **THEN** the doc string SHALL state that it matches
  `cfg.RequirementID` values (Gemara requirement IDs) against
  mapping entries
- **THEN** the doc string SHALL NOT reference "plan RequirementIDs"

### Requirement: ResolveRequirementID doc string accuracy

The `ResolveRequirementID` function doc string in
`cmd/opa-provider/results/results.go` SHALL accurately describe
the ID resolution chain.

#### Scenario: Doc string reflects correct resolution

- **WHEN** the `ResolveRequirementID` function is documented
- **THEN** the doc string SHALL describe resolving Rego-derived IDs
  to Gemara requirement IDs via the reverse mapping

### Requirement: MatchPolicies doc string accuracy

The `MatchPolicies` function doc string in
`cmd/ampel-provider/convert/convert.go` SHALL accurately describe
that it matches requirement IDs from assessment configurations against
granular AMPEL policy IDs.

#### Scenario: Doc string reflects correct field usage

- **WHEN** the `MatchPolicies` function is documented
- **THEN** the doc string SHALL state that it looks up each requirement
  ID (from `AssessmentPlan.RequirementId`) against the granular policy map

### Requirement: Test fixtures populate both PlanID and RequirementID

Test `AssessmentConfiguration` struct literals across all three providers
SHALL populate both `PlanID` and `RequirementID` with distinct values to
reflect the post-complyctl#632 upstream contract.

#### Scenario: OPA test fixtures

- **WHEN** OPA provider tests construct `provider.AssessmentConfiguration`
- **THEN** the struct literal SHALL include both `PlanID` and
  `RequirementID` with different values
- **THEN** `PlanID` SHALL use a plan-style identifier (e.g., `"ap-cis-1"`)
- **THEN** `RequirementID` SHALL use the existing requirement-style
  identifier (e.g., `"CIS-1"`)

#### Scenario: AMPEL test fixtures

- **WHEN** AMPEL provider tests construct `provider.AssessmentConfiguration`
- **THEN** the struct literal SHALL include both `PlanID` and
  `RequirementID` with different values

#### Scenario: OpenSCAP test fixtures

- **WHEN** OpenSCAP provider tests construct `provider.AssessmentConfiguration`
- **THEN** the struct literal SHALL include both `PlanID` and
  `RequirementID` with different values

### Requirement: README ID semantics clarity

Provider README files SHALL clearly distinguish between Gemara assessment
plan IDs (`PlanID`) and Gemara requirement IDs (`RequirementID`).

#### Scenario: OPA README mapping file documentation

- **WHEN** the OPA README documents the `complytime-mapping.json` format
- **THEN** it SHALL state that the `requirement_id` field matches against
  the Gemara assessment plan's `requirement-id` value (not the plan's `id`)
- **THEN** it SHALL NOT use the phrase "Gemara assessment plan RequirementID"

#### Scenario: AMPEL README policy matching documentation

- **WHEN** the AMPEL README documents policy matching
- **THEN** it SHALL clarify that requirement IDs from the Gemara
  `requirement-id` field are matched against granular AMPEL policy IDs

#### Scenario: OpenSCAP README rule mapping documentation

- **WHEN** the OpenSCAP README documents how rules are mapped
- **THEN** it SHALL clarify that `RequirementID` holds XCCDF rule short
  names sourced from the Gemara `requirement-id` field
