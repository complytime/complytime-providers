# Proposal

## Why

All three providers (OpenSCAP, Ampel, OPA) return `AssessmentLog` entries
with an empty `Evidence` slice. complyctl v1.1.0 already ships the full
`Evidence`/`EvidenceSource` proto pipeline and the `MappingReferences`
field on `ScanResponse` (PR #840, PR #882), but no provider populates
them. Without evidence provenance, auditors cannot trace an assessment
result back to the artifact that produced it. This change completes the
evidence traceability chain from provider output to Gemara EvaluationLog.

Additionally, the man pages for all three providers document only the
top-level workspace directory without listing the concrete scan artifacts
(ARF files, attestation files, result JSONs) that the providers produce.
Users and auditors have no documentation explaining what evidence files
exist, how they are created, or how they appear in the EvaluationLog.

## What Changes

- **OpenSCAP**: populate `Evidence` entries referencing the ARF result
  file (`arf.xml`) with SHA256 digest and file path as coordinate.
  Declare one `MappingReference` for the ARF on `ScanResponse`.
- **Ampel**: thread attestation file paths through `RawScanResult` and
  `PerRepoResult` to `ToScanResponse`. Populate two `Evidence` entries
  per assessment: the ampel policy attestation (primary) and the snappy
  observation attestation (supporting), each with digest and coordinate.
  Declare matching `MappingReferences` on `ScanResponse`.
- **OPA**: thread `inputPath` through `PerTargetResult` to
  `ToScanResponse`. Populate one `Evidence` entry per assessment
  referencing the conftest input path as coordinate. Declare matching
  `MappingReferences` on `ScanResponse`.
- **Man pages**: add an `EVIDENCE` section to each provider's man page
  documenting the evidence files produced, how they are created, and
  how they are represented in the Gemara EvaluationLog. Expand the
  `FILES` section with concrete artifact paths.

## Capabilities

### New Capabilities

- `evidence-source`: provider-side population of `Evidence` and
  `Evidence.Source` (EvidenceMapping) fields on `AssessmentLog`,
  plus `MappingReferences` on `ScanResponse`, enabling auditors to
  trace each assessment result to its source artifact.

### Modified Capabilities

*(none -- no existing spec-level behavior changes)*

## Impact

- **Code**: `cmd/openscap-provider/server/`, `cmd/ampel-provider/scan/`,
  `cmd/ampel-provider/results/`, `cmd/opa-provider/results/`,
  `cmd/opa-provider/server/`. Struct changes to `RawScanResult` (Ampel)
  and `PerTargetResult` (OPA) to carry file paths forward.
- **API**: `ScanResponse` will carry `MappingReferences` for the first
  time from these providers. Fully backward-compatible -- complyctl
  merges them with policy-level refs, empty lists are a no-op.
- **Dependencies**: none -- uses only existing complyctl v1.1.0 SDK
  types.
- **Documentation**: three man page markdown sources updated, `.1`
  files regenerated via `make man`.

## Constitution Alignment

- **I. Single Source of Truth**: The shared `internal/evidence/`
  package centralizes digest computation and evidence constants,
  avoiding three copies of identical logic across providers.
- **II. Simplicity & Isolation**: Each provider's evidence population
  is self-contained. The shared utility has a single responsibility
  (file digest computation). Struct additions are minimal and
  internal.
- **III. Incremental Improvement**: This change is focused on a single
  concern (evidence population). No unrelated refactoring.
- **IV. Readability First**: Field names are explicit and descriptive
  (`AmpelAttestationPath`, `SnappyAttestationPath`, `InputPath`,
  `FileDigest`).
- **V. Do Not Reinvent the Wheel**: Uses standard library
  `crypto/sha256`. No new external dependencies.
- **VI. Composability**: Evidence flows through the existing complyctl
  pipeline. `MappingReferences` compose with complyctl's merge logic.
- **VII. Convention Over Configuration**: No new configuration
  introduced. Evidence is populated automatically during scan.
