# Design

## Context

See proposal.md for motivation. The complyctl v1.1.0 SDK already
provides the complete plumbing:

- `provider.Evidence` with `Source *EvidenceSource` (client.go:102-123)
- `provider.MappingReference` (client.go:125-134)
- `ScanResponse.MappingReferences` (client.go:71-77)
- gRPC marshal/unmarshal for both (server.go:108-139, client.go:293-343)

The providers construct `AssessmentLog` at these sites:

| Provider | File                               | Line |
|----------|------------------------------------|------|
| OpenSCAP | `server/server.go`                 | 229  |
| Ampel    | `results/results.go`               | 324  |
| OPA      | `results/results.go`               | 280  |

None currently set the `Evidence` field or return `MappingReferences`.

## Goals / Non-Goals

**Goals:**

- Every `AssessmentLog` carries evidence linking it to the artifact that
  produced the result.
- Each provider's `ScanResponse` declares `MappingReferences` so
  complyctl can merge them into the EvaluationLog metadata.
- Man pages document evidence artifacts and their role in the
  EvaluationLog.

**Non-Goals:**

- Embedding evidence payload inline (file paths via `Source.Coordinate`
  are sufficient; payloads would bloat gRPC messages).
- Deduplicating identical `Evidence` entries across `AssessmentLog`s
  within a single `ScanResponse` (Gemara embeds evidence per
  assessment-log by design; repetition is semantically correct).
- Hashing directory inputs for OPA (fragile, non-reproducible across
  platforms).
- Changing the complyctl SDK or proto definitions.

## Decisions

### D1: Evidence identity scheme

Each `Evidence.ID` uses a deterministic, human-readable string:

| Provider | Pattern                            | Example                                         |
|----------|------------------------------------|--------------------------------------------------|
| OpenSCAP | `"arf"`                            | `"arf"`                                          |
| Ampel    | `"ampel-<prefix>"`                 | `"ampel-github-com-org-repo-main-branch-rules"`  |
| Ampel    | `"snappy-<prefix>"`                | `"snappy-github-com-org-repo-main-branch-rules"` |
| OPA      | `"conftest-<target>[-<branch>]"`   | `"conftest-org/repo-main"`                       |

**Rationale**: IDs must be stable across runs for the same input, and
human-readable for debugging. OpenSCAP has one ARF per scan so a static
ID is sufficient. Ampel and OPA require disambiguation by target.

**Alternative considered**: UUIDs -- rejected because they are not
reproducible and provide no human context.

### D2: MappingReference identity scheme

Each `MappingReference.ID` mirrors the `Evidence.ID` pattern prefixed
with the provider context where needed:

| Provider | MappingReference.ID                | MappingReference.Title                            |
|----------|------------------------------------|---------------------------------------------------|
| OpenSCAP | `"openscap-arf"`                   | `"OpenSCAP ARF scan result"`                      |
| Ampel    | `"ampel-<prefix>"`                 | `"Ampel policy attestation for <repo>@<branch>"`  |
| Ampel    | `"snappy-<prefix>"`                | `"Snappy observation for <repo>@<branch>"`        |
| OPA      | `"conftest-input-<target>[-<branch>]"` | `"Conftest input for <target>[@<branch>]"`    |

`Evidence.Source.ReferenceID` matches the corresponding
`MappingReference.ID` exactly.

**Rationale**: The foreign-key relationship between `EvidenceSource` and
`MappingReference` requires that IDs are identical. Using the same
prefix makes the relationship obvious during debugging.

### D3: Threading attestation paths in Ampel

The attestation file paths are currently computed as local variables
inside `ScanRepository` (`scan.go:285,303`) and lost before reaching
`ToScanResponse`. Two approaches were considered:

- **(a) Add path fields to `RawScanResult` and `PerRepoResult`** --
  carries the paths through the pipeline alongside the existing data.
- **(b) Re-derive paths from naming convention** -- reconstructs the
  paths using `ScanConfig.OutputDir` and the prefix at `ToScanResponse`
  time.

**Decision**: Option (a) -- add fields to the structs. This preserves
single source of truth for path construction (in `ScanRepository`) and
avoids duplicating the naming convention logic.

New fields:

```
RawScanResult {
    Output               []byte   // existing
    AmpelAttestationPath string   // NEW
    SnappyAttestationPath string  // NEW
}
```

```
PerRepoResult {
    ...existing fields...
    AmpelAttestationPath  string  // NEW
    SnappyAttestationPath string  // NEW
}
```

### D4: Threading input path in OPA

Similarly, `inputPath` is available in `processTarget`/`evalAndParse`
but not threaded through to `ToScanResponse`. The `PerTargetResult`
struct gains a new field:

```
PerTargetResult {
    ...existing fields...
    InputPath string  // NEW: path to the directory/file evaluated
}
```

### D5: SHA256 digest computation

OpenSCAP and Ampel evidence entries include `Source.Digest` in
`sha256:<hex>` format. The hash is computed by reading the file after
the scan tool writes it:

- **OpenSCAP**: hash `config.ARFPath` after `oscap` completes and
  before returning the `ScanResponse`.
- **Ampel**: hash both attestation files. The snappy subject hash is
  already extracted in `ScanRepository` (`scan.go:291`) but is of the
  attestation *content*, not the file. Compute file-level SHA256
  separately from the file bytes.
- **OPA**: skip digest -- the input is a directory.

A shared helper function computes the digest:

```go
func fileDigest(path string) (string, error) {
    f, err := os.Open(filepath.Clean(path))
    ...
    h := sha256.New()
    io.Copy(h, f)
    return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}
```

This utility is placed in `internal/evidence/` as it is shared across
providers.

**Alternative considered**: putting it in each provider's package.
Rejected to avoid three copies of identical logic (Constitution
Principle I: Single Source of Truth).

### D6: CollectedAt timestamp

All evidence entries use `time.Now().UTC().Format(time.RFC3339)`. For
OpenSCAP and OPA, the timestamp is captured once at the start of the
scan. For Ampel, it is captured per `ScanRepository` call since each
repo/branch is scanned independently.

### D7: Man page EVIDENCE section placement

The new `EVIDENCE` section is placed between `FILES` and `EXIT CODES`
in each man page. This follows the logical flow: `FILES` describes where
artifacts are stored, `EVIDENCE` explains how those artifacts relate to
the Gemara EvaluationLog, and `EXIT CODES` covers operational behavior.

The `FILES` section is expanded to list concrete artifact paths instead
of just the top-level workspace directory.

### D8: Evidence type values

| Provider | `Evidence.Type`       | Rationale                                  |
|----------|-----------------------|--------------------------------------------|
| OpenSCAP | `"ARF"`               | Asset Reporting Format -- SCAP standard    |
| Ampel    | `"IntotoAttestation"` | in-toto attestation format (DSSE-wrapped)  |
| OPA      | `"ConftestResult"`    | Conftest policy evaluation output          |

These are open-enum values (Gemara `#EvidenceType` accepts any string).
They are descriptive enough to identify the format without being
implementation-specific.

## Risks / Trade-offs

- **[Repetition in evaluation log]** Every `AssessmentLog` from the
  same scan carries the same `Evidence` entries (one ARF file serves
  many rules). This is by design in Gemara's per-assessment-log model
  and accepted as intentional. The `MappingReference` is declared once
  on the `ScanResponse` and merged once into metadata by complyctl.
  *Mitigation*: none needed -- semantically correct.

- **[Struct field additions are technically breaking]** Adding fields to
  `RawScanResult`, `PerRepoResult`, and `PerTargetResult` changes
  their memory layout. However, these are internal types not exposed
  outside their packages.
  *Mitigation*: internal types -- no external consumers.

- **[File hashing adds I/O overhead]** Computing SHA256 of the ARF file
  and attestation files adds a read pass after the scan tool completes.
  ARF files can reach 100+ KB; attestation files are typically <15 KB.
  *Mitigation*: single sequential read per file, negligible compared
  to the scan tool's own runtime.

- **[OPA has no digest]** Conftest evaluates directories. Without a
  digest, evidence integrity cannot be independently verified for OPA.
  *Mitigation*: accepted trade-off -- directory hashing is fragile
  (platform-dependent ordering, git metadata, temp files). The input
  path coordinate is the best available reference.
