# Spec Delta

## Purpose

Ensures that each target in a multi-target policy receives its own
isolated Generate-phase artifacts so that Scan produces correct,
independent compliance results per target.

## ADDED Requirements

### Requirement: Content-addressed Generate artifact storage

The provider SHALL write Generate-phase artifacts (scan-config and, for
Ampel, the merged policy bundle) into a content-keyed subdirectory
derived from a deterministic hash of the sorted matched requirement IDs
and the resolved policy source identifier, rather than a single shared
path.

#### Scenario: Two targets with different requirement sets

- **WHEN** a policy defines two targets that resolve to different
  requirement sets and Generate is called once per target
- **THEN** each Generate call writes its artifacts into a distinct
  content-keyed subdirectory and neither call overwrites the other's
  artifacts

#### Scenario: Two targets with identical requirement sets but different policy sources

- **WHEN** a policy defines two targets that resolve to the same
  requirement set but different policy sources (e.g., different
  `opa_bundle_ref` values or different complypack content) and Generate
  is called once per target
- **THEN** each Generate call writes its artifacts into a distinct
  content-keyed subdirectory because the policy source identifier
  differs, and neither call overwrites the other's artifacts

#### Scenario: Two targets with identical requirement sets and identical policy sources

- **WHEN** a policy defines two targets that resolve to the same
  requirement set and the same policy source and Generate is called once
  per target
- **THEN** both Generate calls write to the same content-keyed
  subdirectory and the second write is an idempotent overwrite of
  identical content

#### Scenario: Deterministic content key

- **WHEN** Generate computes the content key from a set of matched
  requirement IDs and a policy source identifier
- **THEN** the key is the lowercase hex-encoded SHA-256 prefix (first 16
  characters) of the sorted, newline-joined requirement IDs followed by a
  newline and the policy source identifier, and the same input always
  produces the same key

### Requirement: Generate index maps targets to content directories

The provider SHALL maintain a `generate-index.json` file that maps each
target's identifier to its content-key directory, enabling Scan to
locate the correct artifacts per target.

The target identifier SHALL be derived from the target URL when present,
or from the `input_path` value for OPA targets that use local filesystem
paths. The identifier derivation SHALL be collision-resistant: distinct
target identifiers MUST NOT produce the same index key.

#### Scenario: Index created on first Generate call

- **WHEN** Generate is called and no `generate-index.json` exists
- **THEN** Generate creates the file with a single entry mapping the
  current target's identifier to its content key

#### Scenario: Index updated on subsequent Generate calls

- **WHEN** Generate is called and `generate-index.json` already exists
- **THEN** Generate reads the existing index, adds or updates the entry
  for the current target's identifier, and writes the file back

#### Scenario: Index entry updated when target's policy assignment changes

- **WHEN** a target's policy assignment changes between runs and Generate
  is called with the new requirement set
- **THEN** the index entry for that target's identifier is updated to
  point to the new content-key directory

#### Scenario: OPA target with input_path instead of URL

- **WHEN** Generate is called for an OPA target that uses `input_path`
  instead of a URL
- **THEN** the index entry uses a key derived from the `input_path` value
  and the content-key directory is correctly recorded

### Requirement: Scan reads per-target artifacts via the index

The provider SHALL read `generate-index.json` at the start of Scan and
use it to resolve each target's content-key directory independently,
reading scan-config (and, for Ampel, the policy bundle) from that
directory.

#### Scenario: Multi-target Scan with different requirement sets

- **WHEN** Scan is called with two targets that were generated with
  different requirement sets
- **THEN** each target is scanned using the scan-config and policy
  artifacts from its own content-key directory, and the results reflect
  each target's actual requirement set

#### Scenario: Scan graceful degradation without index

- **WHEN** Scan is called and `generate-index.json` does not exist
- **THEN** the provider falls back to the current behavior: OPA first
  tries the legacy shared scan-config.json, then falls back to pulling
  via `opa_bundle_ref`; Ampel reads from the legacy shared path; both
  log a warning

#### Scenario: Scan with unknown target in index

- **WHEN** Scan receives a target whose identifier is not present in
  `generate-index.json`
- **THEN** the provider returns an error identifying the missing target entry
  and MUST NOT read the legacy shared artifact

#### Scenario: Scan with unusable index

- **WHEN** `generate-index.json` cannot be parsed, has an unsupported version,
  or points to unreadable artifacts
- **THEN** the provider returns an error identifying the index condition and
  MUST NOT read the legacy shared artifact

#### Scenario: OPA target with input_path in Scan

- **WHEN** Scan receives an OPA target that uses `input_path` instead of
  a URL
- **THEN** the provider looks up the target's index key using the
  `input_path` value (the same derivation used during Generate) and
  resolves the content-key directory for that target

### Requirement: Content key validation on read

Content keys read from `generate-index.json` SHALL be validated before
use as directory names. The provider SHALL reject content keys that do
not match the expected hex format and SHALL verify that the resolved
path is contained within the generated directory.

#### Scenario: Valid content key from index

- **WHEN** Scan reads a content key from `generate-index.json` that
  matches the expected lowercase hex format (`^[0-9a-f]{16}$`)
- **THEN** the provider resolves the content-key directory path and
  verifies it is within the generated directory before reading artifacts

#### Scenario: Malformed content key in index

- **WHEN** Scan reads a content key from `generate-index.json` that does
  not match the expected hex format or that resolves to a path outside the
  generated directory (e.g., path traversal via `../`)
- **THEN** the provider returns an error identifying the invalid content key
  and MUST NOT read the legacy shared artifact

### Requirement: Per-target result aggregation

The provider SHALL associate each target's scan results with the reverse
mapping (OPA) or requirement-ID list (Ampel) from that target's own
scan-config, rather than using a single provider-wide mapping for all
targets.

#### Scenario: OPA multi-target result aggregation

- **WHEN** Scan processes two OPA targets with different reverse mappings
  (because their scan-configs contain different `reverse_mapping` values)
- **THEN** each target's conftest results are resolved using only that
  target's reverse mapping, and synthetic passing assessments are
  generated per target using only that target's mapping values

#### Scenario: Ampel multi-target result aggregation

- **WHEN** Scan processes two Ampel targets with different requirement-ID
  lists (because their scan-configs contain different `requirement_ids`)
- **THEN** each target's scan results are matched only against that
  target's requirement-ID list, and synthetic passing assessments are not
  generated for requirement IDs that do not apply to a given target

### Requirement: Uniform approach across policy source types

The content-addressed index approach SHALL apply uniformly regardless of
whether the policy source is an OCI bundle reference (`opa_bundle_ref`),
a complypack (`ComplypackContentPath`), a custom directory
(`ampel_policy_dir`), or the default granular policy path.

#### Scenario: OPA with opa_bundle_ref

- **WHEN** Generate resolves the policy source via `opa_bundle_ref`
- **THEN** artifacts are written to a content-keyed subdirectory under
  the provider's generated directory and the index is updated

#### Scenario: OPA with ComplypackContentPath

- **WHEN** Generate resolves the policy source via `ComplypackContentPath`
- **THEN** artifacts are written to a content-keyed subdirectory under
  the provider's generated directory (not the complypack cache) and the
  index is updated

#### Scenario: Ampel with any policy source

- **WHEN** Ampel's Generate resolves granular policies from any supported
  source (complypack, custom directory, or default path)
- **THEN** both the merged policy bundle and scan-config are written to a
  content-keyed subdirectory under the provider's generated directory and
  the index is updated

### Requirement: Concurrency safety annotation

The `generate-index.json` read-modify-write cycle SHALL include a
comment in the source code noting that concurrent complyctl instances
sharing the same workspace may race on this file, and that file-level
locking is a future consideration.

#### Scenario: Sequential Generate calls

- **WHEN** complyctl calls Generate sequentially (one target at a time)
- **THEN** the index file is updated correctly with no data loss
