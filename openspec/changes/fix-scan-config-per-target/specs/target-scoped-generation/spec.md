# Spec Delta

## Purpose

Ensures that each target in a multi-target policy receives its own
isolated Generate-phase artifacts so that Scan produces correct,
independent compliance results per target.

## ADDED Requirements

### Requirement: Content-addressed Generate artifact storage

The provider SHALL write Generate-phase artifacts (scan-config and, for
Ampel, the merged policy bundle) into a content-keyed subdirectory
derived from a deterministic hash of the sorted matched requirement IDs,
rather than a single shared path.

#### Scenario: Two targets with different requirement sets

- **WHEN** a policy defines two targets that resolve to different
  requirement sets and Generate is called once per target
- **THEN** each Generate call writes its artifacts into a distinct
  content-keyed subdirectory and neither call overwrites the other's
  artifacts

#### Scenario: Two targets with identical requirement sets

- **WHEN** a policy defines two targets that resolve to the same
  requirement set and Generate is called once per target
- **THEN** both Generate calls write to the same content-keyed
  subdirectory and the second write is an idempotent overwrite of
  identical content

#### Scenario: Deterministic content key

- **WHEN** Generate computes the content key from a set of matched
  requirement IDs
- **THEN** the key is the lowercase hex-encoded SHA-256 prefix (first 16
  characters) of the sorted, newline-joined requirement IDs, and the same
  input always produces the same key

### Requirement: Generate index maps targets to content directories

The provider SHALL maintain a `generate-index.json` file that maps each
target's sanitized URL to its content-key directory, enabling Scan to
locate the correct artifacts per target.

#### Scenario: Index created on first Generate call

- **WHEN** Generate is called and no `generate-index.json` exists
- **THEN** Generate creates the file with a single entry mapping the
  current target's sanitized URL to its content key

#### Scenario: Index updated on subsequent Generate calls

- **WHEN** Generate is called and `generate-index.json` already exists
- **THEN** Generate reads the existing index, adds or updates the entry
  for the current target's sanitized URL, and writes the file back

#### Scenario: Index entry updated when target's policy assignment changes

- **WHEN** a target's policy assignment changes between runs and Generate
  is called with the new requirement set
- **THEN** the index entry for that target's URL is updated to point to
  the new content-key directory

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
- **THEN** the provider falls back to the current behavior (reading from
  the shared legacy path or pulling bundles directly) and logs a warning

#### Scenario: Scan with unknown target in index

- **WHEN** Scan receives a target whose sanitized URL is not present in
  `generate-index.json`
- **THEN** the provider falls back to pulling the bundle directly (OPA)
  or logs a warning and skips passing-assessment synthesis (Ampel), and
  does not return an error for the entire Scan call

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
