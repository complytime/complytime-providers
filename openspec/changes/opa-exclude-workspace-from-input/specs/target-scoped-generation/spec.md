# Spec Delta

## MODIFIED Requirements

### Requirement: Per-target result aggregation

The provider SHALL associate each target's scan results with the reverse
mapping (OPA) or requirement-ID list (Ampel) from that target's own
scan-config, rather than using a single provider-wide mapping for all
targets. An OPA local or remote directory target without confirmed evaluated
eligible input SHALL produce a target-level operational error and SHALL NOT produce
assessments, including synthetic passing assessments. This does not suppress
results from other targets. If any selected branch of a configured remote target
fails input-safety preflight or has no confirmed evaluated eligible directory input,
the provider SHALL suppress assessments from every branch of that configured
target, regardless of branch order, while retaining its operational errors and
results from other configured targets.

#### Scenario: OPA multi-target result aggregation

- **WHEN** Scan processes two OPA targets with different reverse mappings
  (because their scan-configs contain different `reverse_mapping` values)
  and both targets have evaluated eligible input
- **THEN** each target's conftest results are resolved using only that
  target's reverse mapping, and synthetic passing assessments are
  generated per target using only that target's mapping values

#### Scenario: OPA directory target with no confirmed evaluated input

- **WHEN** an OPA local or remote directory target has no confirmed evaluated eligible input
- **THEN** Scan reports a target-level operational error and generates no
  assessments for that target, including synthetic passing assessments
- **AND** other targets retain their own results and assessments

#### Scenario: OPA multi-branch target partially rejected

- **GIVEN** one branch of a configured OPA remote target evaluates successfully
- **AND** another branch fails input-safety preflight or lacks confirmed evaluated directory input
- **WHEN** Scan aggregates results in either branch order
- **THEN** it reports the rejected target and branch's operational error
- **AND** it generates no assessments from any branch of that configured target
- **AND** it retains successful assessments and mapping references from other configured targets

#### Scenario: Ampel multi-target result aggregation

- **WHEN** Scan processes two Ampel targets with different requirement-ID
  lists (because their scan-configs contain different `requirement_ids`)
- **THEN** each target's scan results are matched only against that
  target's requirement-ID list, and synthetic passing assessments are not
  generated for requirement IDs that do not apply to a given target
