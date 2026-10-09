# Spec Delta

## Purpose

Enables providers to report evidence provenance on each assessment log
so that auditors can trace a compliance result back to the artifact that
produced it, and ensures the provider man pages document every evidence
artifact the provider creates.

## ADDED Requirements

### Requirement: Evidence entry per assessment log

Each provider MUST populate at least one `Evidence` entry on every
`AssessmentLog` it returns in a `ScanResponse`.

#### Scenario: OpenSCAP returns ARF evidence

- **GIVEN** the OpenSCAP provider has successfully executed `oscap
  xccdf eval` and produced an ARF file
- **WHEN** the provider constructs the `ScanResponse`
- **THEN** every `AssessmentLog` in the response contains an `Evidence`
  entry with `Type` set to `"ARF"` and a non-empty `CollectedAt`
  timestamp in RFC 3339 format

#### Scenario: Ampel returns two attestation evidence entries

- **GIVEN** the Ampel provider has successfully scanned a repository
- **WHEN** the provider constructs the `ScanResponse`
- **THEN** every `AssessmentLog` from that repository contains two
  `Evidence` entries: one with `ID` prefixed `"ampel-"` and one with
  `ID` prefixed `"snappy-"`, both with `Type` set to
  `"IntotoAttestation"` and non-empty `CollectedAt` timestamps in
  RFC 3339 format

#### Scenario: OPA returns conftest evidence

- **GIVEN** the OPA provider has successfully evaluated a target
- **WHEN** the provider constructs the `ScanResponse`
- **THEN** every `AssessmentLog` in the response contains an `Evidence`
  entry with `Type` set to `"ConftestResult"` and a non-empty
  `CollectedAt` timestamp in RFC 3339 format

### Requirement: EvidenceSource coordinate references a file path

Each `Evidence` entry MUST have a non-nil `Source` with `Coordinate`
set to the file system path of the evidence artifact.

#### Scenario: OpenSCAP coordinate points to ARF file

- **GIVEN** a completed OpenSCAP scan with an ARF file on disk
- **WHEN** an OpenSCAP `Evidence` entry is returned
- **THEN** `Source.Coordinate` equals the ARF file path under the
  provider workspace (`.complytime/openscap/results/arf.xml`)

#### Scenario: Ampel coordinates point to attestation files

- **GIVEN** a completed Ampel scan with attestation files on disk
- **WHEN** Ampel `Evidence` entries are returned for a repo/branch/spec
  combination
- **THEN** the ampel evidence `Source.Coordinate` ends with
  `-ampel.intoto.json` and the snappy evidence `Source.Coordinate` ends
  with `-snappy.intoto.json`

#### Scenario: OPA coordinate points to evaluated input

- **GIVEN** a completed OPA evaluation with an input path resolved
- **WHEN** an OPA `Evidence` entry is returned
- **THEN** `Source.Coordinate` equals the input path that was evaluated
  by conftest (cloned repository directory or local file path)

### Requirement: EvidenceSource digest for file-based evidence

When the evidence artifact is a single file, `Source.Digest` MUST
contain a SHA256 hash in `sha256:<hex>` format.

#### Scenario: OpenSCAP digest covers ARF file

- **GIVEN** a completed OpenSCAP scan with an ARF file on disk
- **WHEN** an OpenSCAP `Evidence` entry is returned
- **THEN** `Source.Digest` matches the pattern `sha256:[a-f0-9]{64}`
  and equals the SHA256 hash of the ARF file content at scan time

#### Scenario: Ampel digest covers attestation files

- **GIVEN** a completed Ampel scan with attestation files on disk
- **WHEN** Ampel `Evidence` entries are returned
- **THEN** both the ampel and snappy evidence entries have
  `Source.Digest` matching the pattern `sha256:[a-f0-9]{64}`

#### Scenario: OPA omits digest for directory inputs

- **GIVEN** an OPA evaluation against a directory input
- **WHEN** an OPA `Evidence` entry references that directory
- **THEN** `Source.Digest` is empty (hashing a directory tree is
  impractical and fragile)

#### Scenario: Digest computation failure returns an error

- **GIVEN** a scan has completed but the evidence artifact file is
  unreadable (missing, permission denied, or I/O error)
- **WHEN** the provider attempts to compute the file digest
- **THEN** the provider returns an error on the `ScanResponse` rather
  than returning evidence with an empty or partial digest

### Requirement: EvidenceSource ReferenceID links to MappingReference

Each `Evidence.Source.ReferenceID` MUST match the `ID` of a
`MappingReference` returned on the same `ScanResponse`.

Note: `Evidence.ID` is provider-internal and need not match
`MappingReference.ID`. Only `Evidence.Source.ReferenceID` is required
to match `MappingReference.ID` (the foreign-key relationship).

#### Scenario: OpenSCAP declares ARF mapping reference

- **GIVEN** a completed OpenSCAP scan with evidence attached
- **WHEN** the OpenSCAP provider returns a `ScanResponse`
- **THEN** `MappingReferences` contains an entry with `ID` equal to
  `"openscap-arf"` and `Evidence.Source.ReferenceID` on each assessment
  log matches that ID

#### Scenario: Ampel declares attestation mapping references

- **GIVEN** a completed Ampel scan with evidence attached
- **WHEN** the Ampel provider returns a `ScanResponse`
- **THEN** `MappingReferences` contains entries whose `ID` values match
  the `ReferenceID` values on every `Evidence.Source` in the response

#### Scenario: OPA declares input mapping reference

- **GIVEN** a completed OPA evaluation with evidence attached
- **WHEN** the OPA provider returns a `ScanResponse`
- **THEN** `MappingReferences` contains entries whose `ID` values match
  the `ReferenceID` values on every `Evidence.Source` in the response

### Requirement: Man page documents evidence artifacts

Each provider's man page MUST include an `EVIDENCE` section that
describes the evidence files the provider produces, how they are
created, and how they appear in the Gemara EvaluationLog.

#### Scenario: OpenSCAP man page describes ARF evidence

- **WHEN** a user reads the `complyctl-provider-openscap(1)` man page
- **THEN** they find an `EVIDENCE` section describing the ARF file, its
  location, how it is produced by `oscap xccdf eval`, and that it
  appears as `Evidence` with `Source.Coordinate` in the EvaluationLog

#### Scenario: Ampel man page describes attestation evidence

- **WHEN** a user reads the `complyctl-provider-ampel(1)` man page
- **THEN** they find an `EVIDENCE` section describing both snappy and
  ampel attestation files, their naming convention, and their
  representation in the EvaluationLog

#### Scenario: OPA man page describes conftest evidence

- **WHEN** a user reads the `complyctl-provider-opa(1)` man page
- **THEN** they find an `EVIDENCE` section describing the conftest input
  path evidence, and its representation in the EvaluationLog

### Requirement: Expanded FILES section in man pages

Each provider's man page `FILES` section MUST list the concrete
artifact paths the provider creates, not only the top-level workspace
directory.

#### Scenario: OpenSCAP FILES lists scan artifacts

- **WHEN** a user reads the `complyctl-provider-openscap(1)` man page
- **THEN** the `FILES` section lists `arf.xml`, `results.xml`,
  `tailoring.xml`, and the remediation script paths

#### Scenario: Ampel FILES lists attestation artifacts

- **WHEN** a user reads the `complyctl-provider-ampel(1)` man page
- **THEN** the `FILES` section lists the attestation file naming
  pattern, per-repo result JSON pattern, and `scan-config.json`

#### Scenario: OPA FILES lists scan artifacts

- **WHEN** a user reads the `complyctl-provider-opa(1)` man page
- **THEN** the `FILES` section lists the per-target result JSON pattern,
  `scan-config.json`, `complytime-mapping.json`, the clone directory,
  and the generated directory
