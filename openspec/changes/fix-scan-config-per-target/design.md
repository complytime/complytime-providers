# Design

## Context

See proposal.md for motivation. The current architecture is:

- `complyctl` calls `Generate` once per `(evaluator, target)` pair,
  passing a flat `TargetVariables map[string]string]` (no target ID).
- `complyctl` calls `Scan` once per evaluator with all targets batched
  in a `[]Target` slice (each carrying `TargetID` and `Variables`).
- Both providers write `scan-config.json` to a single shared path.
  Ampel additionally writes a merged policy bundle to the same shared
  directory.
- `ScanRequest` carries no requirement IDs and no
  `ComplypackContentPath` — `Scan` depends entirely on artifacts
  written by `Generate` to reconstruct what to evaluate.

Key constraints:
- `GenerateRequest` has no `TargetID` field (cannot scope by target).
- `ScanRequest` has no `ComplypackContentPath` field (cannot
  re-derive the complypack path at Scan time).
- The `TargetVariables["url"]` field is available at both Generate
  and Scan time and is unique per target.
- File sizes are small: scan-config < 5 KB, merged Ampel bundle
  50-200 KB.
- Hundreds of targets sharing the same policy set is a realistic
  deployment pattern.

## Goals / Non-Goals

**Goals:**
- Eliminate silent data corruption when multiple targets produce
  different Generate artifacts.
- Use a unified approach across both providers and all policy source
  types (OCI bundle, complypack, custom directory, default path).
- Avoid duplicating content when many targets share the same
  requirement set.
- Maintain backward compatibility for single-target policies.

**Non-Goals:**
- Modifying `complyctl`'s plugin interface (no changes to
  `GenerateRequest` or `ScanRequest` proto definitions).
- Adding file-level locking for concurrent workspace access
  (documented as future consideration).
- Cleaning up stale content directories from previous runs
  (accepted as harmless; tracked for future improvement).
- Changing the OpenSCAP provider (it has a different architecture
  and is not affected by this bug).

## Decisions

### Decision 1: Content-addressed directories keyed by requirement hash

**Choice**: Derive a content key from a SHA-256 hash of sorted
matched requirement IDs. Write all Generate artifacts into a
subdirectory named by that key.

**Why over per-target-URL directories**: With hundreds of targets
sharing the same policy, per-URL directories would duplicate
identical artifacts hundreds of times (up to ~50 MB for 500 targets
with a 100 KB merged bundle). Content-addressing deduplicates
naturally — targets with identical requirement sets share one
directory.

**Why over per-bundle-ref directories (OPA's PolicyDirForBundle)**:
`PolicyDirForBundle` only works for the `opa_bundle_ref` path. The
`ComplypackContentPath` path has no equivalent key at Scan time.
A unified content-addressed approach handles all source types with
one mechanism.

**Key derivation**: Sort requirement IDs lexicographically, join
with newlines, SHA-256 hash, take the first 16 hex characters. This
provides sufficient collision resistance for the expected namespace
(hundreds to low thousands of distinct requirement sets) while
keeping directory names readable in logs.

**Alternatives rejected**:
- Full SHA-256 (64 chars): unnecessarily long directory names.
- Hash of entire ScanConfig JSON: sensitive to non-semantic
  differences (timestamps, field ordering).
- Sequential numbering: not deterministic across runs.

### Decision 2: generate-index.json maps targets to content keys

**Choice**: Maintain a JSON file mapping sanitized target URLs to
content-key directories. Generate reads, updates, and writes this
file on each call. Scan reads it once at the start and uses it to
resolve per-target artifact directories.

**Why a mapping file**: At Scan time, the provider has
`target.Variables["url"]` but not the requirement IDs that determine
the content key. The index bridges this gap without requiring
changes to `ScanRequest`.

**File location**: In the provider's generated directory
(`GeneratedDirPath()` for OPA, `ScanConfigDirPath()` for Ampel).
This keeps the index alongside the content directories it
references.

**Schema**:
```json
{
  "version": 1,
  "targets": {
    "<sanitized-url>": "<content-key>",
    ...
  }
}
```

The `version` field enables future schema evolution without breaking
existing workspaces.

**Concurrency note**: The read-modify-write cycle on
`generate-index.json` is safe for sequential Generate calls (the
current complyctl behavior). Concurrent complyctl instances sharing
the same workspace could race on this file. A source code comment
documents this assumption. File-level locking is a non-goal for
this change.

### Decision 3: Artifacts written to generated directory, not policy directory

**Choice**: Content-keyed directories live under the provider's
generated directory (OPA: `.complytime/opa/generated/<key>/`,
Ampel: `.complytime/ampel/generated/<key>/`), not under the policy
directory.

**Why**: The policy directory (`policy/` for both providers) holds
downloaded or source content (OCI bundles, granular policy files).
The generated directory holds derived artifacts. Keeping
content-keyed output in `generated/` preserves this semantic
separation. For OPA, `policy/<sanitized-bundle>/` continues to hold
the pulled Rego files; the scan-config for that bundle lives in
`generated/<content-key>/`. For Ampel, `granular-policies/` holds
source files; the merged bundle and scan-config live in
`generated/<content-key>/`.

**Change for Ampel**: Ampel currently has no `generated/` directory.
This design adds one (via `EnsureDirectories`), mirroring OPA's
structure. The existing `policy/` directory that served as Ampel's
output path is repurposed to hold only the `generate-index.json`
and content-keyed subdirectories, or a new `generated/` sibling is
introduced.

### Decision 4: OPA scan-config stores policyDir in BundleDir field

**Choice**: The OPA `ScanConfig.BundleDir` field continues to store
the resolved policy directory path (whether from
`PolicyDirForBundle` or complypack extraction). This is the only
mechanism for Scan to locate the Rego files — especially for
complypacks, where the path cannot be re-derived from
`ScanRequest`.

**Why**: `BundleDir` is what bridges the Generate-time policy
resolution to Scan-time policy location. Moving scan-config into a
content-keyed directory does not eliminate the need for this field.

### Decision 5: Graceful fallback for missing index

**Choice**: When `generate-index.json` is missing or a target's URL
is not found in the index, Scan falls back to current behavior:
- OPA: pulls the bundle via `opa_bundle_ref` from
  `target.Variables` (the existing no-scan-config fallback).
- Ampel: skips passing-assessment synthesis and logs a warning (the
  existing no-scan-config fallback).

**Why**: This preserves backward compatibility with workspaces
created before this change and with Scan-without-Generate workflows.

## Risks / Trade-offs

**[Stale content directories]** → Content-keyed directories from
previous runs may linger when a target's policy assignment changes.
This wastes disk space but does not affect correctness (the index
always points to the current directory). Accepted as harmless;
cleanup can be added in a future change.

**[Index file as single point of failure]** → If
`generate-index.json` is corrupted or deleted, Scan falls back to
the no-index path. For OPA this means re-pulling bundles; for Ampel
this means degraded output. The fallback is the same behavior as
today, so this is not a regression.

**[Hash collision]** → 16 hex characters = 64 bits of entropy. The
birthday bound for 50% collision probability is ~2^32 distinct
requirement sets (~4 billion). This is far beyond realistic usage.
Not a practical risk.

**[Ampel generated directory is new]** → Adding `generated/` to
Ampel's `EnsureDirectories` is a visible filesystem change. Users
inspecting the workspace will see a new directory. This is
cosmetic and does not affect functionality.

**[URL as index key assumes unique URLs per target]** → Two targets
with the same URL but different policy assignments would map to the
same index entry, and the last Generate call wins. This is
documented as an unsupported configuration — the correct approach is
one target with multiple policies. This can be validated upstream in
`complyctl`.
