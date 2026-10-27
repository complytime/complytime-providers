# Spec Delta

## Purpose

Handle ampel SKIP evaluation status in the ampel provider so that
multi-predicate-type policy bundles produce correct per-policy results
instead of misreporting non-applicable policies as failures.

## ADDED Requirements

### Requirement: SKIP eval results propagate as skip findings
The provider SHALL parse ampel eval results with status `"SKIP"` and
produce findings with result `"skip"` instead of `"fail"`.

#### Scenario: Single policy SKIPs due to non-matching predicate type
- **WHEN** ampel returns a ResultSet where one policy has eval_results
  with status `"SKIP"` and another has status `"PASS"`
- **THEN** the parsed PerRepoResult contains a `"skip"` finding for
  the first policy and a `"pass"` finding for the second

#### Scenario: SKIP eval result carries assessment message
- **WHEN** ampel returns a SKIP eval result with an assessment message
- **THEN** the finding's Reason field contains the assessment message

### Requirement: SKIP overall status propagates as skip
The provider SHALL parse ampel overall predicate status `"SKIP"` and
set the PerRepoResult status to `"skip"` instead of `"fail"`.

#### Scenario: Overall status is SKIP
- **WHEN** ampel returns a ResultSet with overall predicate status `"SKIP"`
- **THEN** the parsed PerRepoResult has status `"skip"`

### Requirement: Provider passes --allow-skip to ampel verify
The provider SHALL include `--allow-skip` in the ampel verify CLI
arguments so that root-level SKIP is not normalized to FAIL by ampel.

#### Scenario: Constructed ampel verify command includes --allow-skip
- **WHEN** the provider constructs the ampel verify command
- **THEN** the argument list contains `"--allow-skip"`

### Requirement: All-SKIP results are treated as error
The provider SHALL treat a PerRepoResult where every finding has result
`"skip"` (and at least one finding exists) as an error condition. At
least one policy MUST produce PASS or FAIL for a valid evaluation.

#### Scenario: All findings are SKIP
- **WHEN** ampel returns a ResultSet where every policy produces SKIP
  and no policy produces PASS or FAIL
- **THEN** the provider sets the PerRepoResult status to `"error"` with
  a message indicating no policies matched the attestation predicate type

#### Scenario: Mixed SKIP and PASS results are valid
- **WHEN** ampel returns a ResultSet with some policies producing SKIP
  and at least one producing PASS or FAIL
- **THEN** the provider preserves each finding's result as-is and does
  not convert to error

### Requirement: Version guard ensures ampel >= 1.4.2
The provider SHALL verify that the installed ampel CLI is version 1.4.2
or later before executing scans. This prevents confusing partial
failures from the unrecognized `--allow-skip` flag on older versions.

#### Scenario: ampel version meets minimum requirement
- **WHEN** `ampel version` reports a version >= 1.4.2
- **THEN** `checkRequiredTools` succeeds and the scan proceeds

#### Scenario: ampel version is below minimum requirement
- **WHEN** `ampel version` reports a version < 1.4.2
- **THEN** `checkRequiredTools` returns an actionable error indicating
  the minimum required ampel version

### Requirement: WhenClause field in AmpelPolicy supports round-trip
The AmpelPolicy struct SHALL include a `When` field so that policies
with `when:` guards can be loaded, merged, and written without losing
the field.

#### Scenario: Policy with when guard round-trips through load/merge/write
- **WHEN** a granular policy JSON file contains a `"when"` field with
  an `"expression"` value
- **THEN** the loaded, merged, and written policy bundle preserves the
  `"when"` field with the same expression value
