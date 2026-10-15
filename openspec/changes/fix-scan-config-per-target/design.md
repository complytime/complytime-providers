# Design

## Context

See proposal.md for motivation. The current architecture is:

- `complyctl` calls `Generate` once per `(evaluator, target)` pair,
  passing a flat `TargetVariables map[string]string` (no target ID).
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

### Decision 1: Content-addressed directories keyed by requirement + policy source hash

**Choice**: Derive a content key from a SHA-256 hash of sorted
matched requirement IDs and the resolved policy source identifier.
Write all Generate artifacts into a subdirectory named by that key.

**Why policy source is included in the hash**: Requirement IDs alone
do not fully identify the generated artifacts. OPA artifacts also
depend on the reverse mapping and the policy directory (different
`opa_bundle_ref` values or complypack content produce different Rego
files). Ampel bundles depend on the granular policy source content.
Two targets with identical requirement IDs but different policy
sources would produce different artifacts. Hashing only requirement
IDs would assign them the same content key, causing one to overwrite
the other — reproducing the exact bug this change fixes.

**Policy source identifier by provider**:
- **OPA**: the resolved policy directory path (the absolute path
  after `resolvePolicyDir` — whether from `ComplypackContentPath`,
  `opa_bundle_ref` via `PolicyDirForBundle`, or custom directory).
  This captures the actual policy content location regardless of
  source type.
- **Ampel**: the resolved granular policy source directory path
  (from `ComplypackContentPath`, `ampel_policy_dir`, or the default
  `GranularPolicyDirPath()`).

**Why over per-target-URL directories**: With hundreds of targets
sharing the same policy, per-URL directories would duplicate
identical artifacts hundreds of times (up to ~50 MB for 500 targets
with a 100 KB merged bundle). Content-addressing deduplicates
naturally — targets with identical requirement sets and policy
sources share one directory.

**Why over per-bundle-ref directories (OPA's PolicyDirForBundle)**:
`PolicyDirForBundle` only works for the `opa_bundle_ref` path. The
`ComplypackContentPath` path has no equivalent key at Scan time.
A unified content-addressed approach handles all source types with
one mechanism.

**Key derivation**: Sort requirement IDs lexicographically, join
with newlines, append a newline and the policy source identifier,
SHA-256 hash the result, take the first 16 hex characters. This
provides sufficient collision resistance for the expected namespace
(hundreds to low thousands of distinct requirement-and-source
combinations) while keeping directory names readable in logs.

**Alternatives rejected**:
- Full SHA-256 (64 chars): unnecessarily long directory names.
- Hash of entire ScanConfig JSON: sensitive to non-semantic
  differences (timestamps, field ordering).
- Sequential numbering: not deterministic across runs.
- Hash of requirement IDs only: does not distinguish different
  policy sources with the same requirement set (see above).

### Decision 2: generate-index.json maps targets to content keys

**Choice**: Maintain a JSON file mapping target keys
(collision-resistant hashes derived from target URLs or input paths)
to content-key directories. Generate reads, updates, and writes
this file on each call. Scan reads it once at the start and uses it
to resolve per-target artifact directories.

**Why a mapping file**: At Scan time, the provider has
`target.Variables["url"]` but not the requirement IDs that determine
the content key. The index bridges this gap without requiring
changes to `ScanRequest`.

**File location**: In the provider's generated directory
(`GeneratedDirPath()` for both OPA and Ampel). This keeps the
index alongside the content directories it references.

**Schema**:
```json
{
  "version": 1,
  "targets": {
    "<target-key>": "<content-key>",
    ...
  }
}
```

The `version` field enables future schema evolution without breaking
existing workspaces. When Scan reads an index with a `version` value
other than `1`, it SHALL log a warning and fall back to the no-index
behavior.

**Target identifier derivation**: Index keys are derived from
target identifiers using a `TargetKey` function in the shared
`internal/genindex` package (see task 1.4). The function accepts
either a URL or an `input_path` value and produces a
collision-resistant key:
- For URL targets: parse with `net/url.Parse`, strip scheme, strip
  `userinfo@` (credentials), then hash the normalized result
  (SHA-256 prefix, 16 hex chars). This prevents credential leakage
  and ensures URLs with and without embedded credentials produce the
  same key. Hashing instead of character replacement avoids the
  lossy sanitization problem where distinct URLs (e.g.,
  `github.com/org/repo` and `github.com:org/repo`) could collide.
- For `input_path` targets (OPA only): `filepath.Clean` the path,
  then hash it with the same SHA-256 prefix approach.
Both providers currently implement identical copies of a lossy
`SanitizeRepoURL` in their respective `targets` packages; this
change replaces those with the collision-resistant `TargetKey`.
The existing `SanitizeRepoURL` functions are retained for their
current non-index uses (result file naming) where human readability
matters and collisions are acceptable.

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
This design adds a new `generated/` sibling directory (via
`EnsureDirectories`), mirroring OPA's structure. The existing
`policy/` directory (`GeneratedPolicyDirPath()`) retains its
current role as the legacy output path; the new `generated/`
directory holds the `generate-index.json` and content-keyed
subdirectories.

### Decision 4: OPA scan-config stores policyDir in PolicyDir field

**Choice**: Rename `ScanConfig.BundleDir` to `ScanConfig.PolicyDir`
(JSON tag `"policy_dir"`). The field stores the resolved policy
directory path regardless of source type (OCI bundle via
`PolicyDirForBundle`, complypack extraction, or custom directory).
This is the only mechanism for Scan to locate the Rego files —
especially for complypacks, where the path cannot be re-derived
from `ScanRequest`.

**Why rename**: The field has always stored a general policy
directory path, not specifically a bundle directory. Since this
change already restructures how scan-config is written and read
(moving it into content-keyed directories), the JSON schema is
effectively evolving — this is the natural time to fix the
misleading name. The rename is contained within `cmd/opa-provider/`
(Ampel has no equivalent field). Old workspaces with `"bundle_dir"`
in their scan-config hit the Decision 5 fallback path (no index →
legacy behavior), so backward compatibility is preserved without
a JSON alias.

**Why the field is still needed**: `PolicyDir` bridges the
Generate-time policy resolution to Scan-time policy location.
Moving scan-config into a content-keyed directory does not
eliminate the need for this field.

### Decision 5: Graceful fallback for missing index

**Choice**: When `generate-index.json` is missing or a target's
identifier is not found in the index, Scan falls back with a
two-step chain:
- OPA: (1) try reading the legacy shared `scan-config.json` from
  the flat `GeneratedDirPath()` — this covers complypack workspaces
  where the policy came from `ComplypackContentPath` and has no
  `opa_bundle_ref` to fall back to; (2) if not found, pull the
  bundle via `opa_bundle_ref` from `target.Variables` (the existing
  no-scan-config fallback). Pulling via `opa_bundle_ref` alone is
  insufficient for complypack workspaces because `ScanRequest` does
  not carry `ComplypackContentPath` — the resolved policy directory
  is only available in the scan-config written by Generate.
- Ampel: reads from the legacy shared path
  (`ScanConfigDirPath()`), logs a warning, and skips
  passing-assessment synthesis if scan-config is not found.

**Why**: This preserves backward compatibility with workspaces
created before this change, with Scan-without-Generate workflows,
and critically with legacy complypack workspaces where no
`opa_bundle_ref` exists.

### Decision 6: Content key validation on read

**Choice**: When Scan reads a content key from `generate-index.json`,
validate that it matches the expected hex format (`^[0-9a-f]{16}$`)
and that the resolved absolute path is contained within the generated
directory (no path traversal). Invalid keys trigger a fallback to the
no-index behavior for that target.

**Why**: A modified or corrupted `generate-index.json` could contain
a content key with path traversal components (e.g., `../../etc`).
The project already establishes path traversal protection as a
pattern in `internal/archive/` (aggregate size limits, file count
limits, and `filepath.Rel` containment checks). Applying the same
defense to content keys maintains consistent security posture.

### Decision 7: Per-target result aggregation

**Choice**: Thread per-target scan-config data through to the
result aggregation step so each target's results use only that
target's reverse mapping (OPA) or requirement-ID list (Ampel).

**OPA**: Currently `ToScanResponse(allResults, reverseMap)` receives
a single `reverseMap` from the one shared scan-config. With
per-target scan-configs, each target may have a different
`ReverseMapping`. The change associates each `PerTargetResult` with
the reverse mapping from its own scan-config. `ToScanResponse` (or
a restructured equivalent) uses the per-result mapping for ID
resolution and per-target mappings for synthetic passing assessment
generation.

**Ampel**: Currently `ToScanResponse(repoResults, allRequirementIDs)`
receives a single `allRequirementIDs` list from the one shared
scan-config. With per-target scan-configs, each target may have a
different `requirement_ids` list. The change associates each
`PerRepoResult` with the requirement-ID list from its own
scan-config. The response builder uses per-result requirement IDs
so that synthetic passing assessments are not generated for
requirement IDs that do not apply to a given target.

**Why**: Without this change, reading per-target scan-configs in the
Scan loop solves the artifact location problem but not the result
correctness problem. The last target's reverse mapping or
requirement-ID list would still overwrite earlier ones, producing
incorrect synthetic passing assessments for targets whose
requirements differ.

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
requirement-set-and-source combinations (~4 billion). This is far
beyond realistic usage. Not a practical risk.

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
