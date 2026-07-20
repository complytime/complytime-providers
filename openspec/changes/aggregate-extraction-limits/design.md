## Context

The ampel provider (`cmd/ampel-provider/server/unpack.go`) and OPA provider (`cmd/opa-provider/server/server.go`, lines 218-355) contain functionally identical tar.gz extraction implementations: `resolveComplypackPath`, `extractTarGz`, and `writeFileFromTar`. Both cap individual files at 100 MB but enforce no aggregate limits on total extracted bytes or file count.

Issue #71 requests adding aggregate limits. Applying the fix to both copies independently would violate the constitution's Single Source of Truth principle and guarantee future drift (as already evidenced by the OPA provider's missing partial-extraction cleanup).

Current code locations:

| Provider | File | Functions |
|----------|------|-----------|
| Ampel | `cmd/ampel-provider/server/unpack.go` (lines 1-159) | `resolveComplypackPath`, `extractTarGz`, `writeFileFromTar` |
| OPA | `cmd/opa-provider/server/server.go` (lines 218-355) | Same three functions (copy-pasted) |

Differences between the two copies:
1. Comment wording: "policy files (JSON)" vs "policy files (Rego, JSON mapping)"
2. Ampel's `resolveComplypackPath` calls `os.RemoveAll` on failed extraction; OPA's does not

## Goals / Non-Goals

### Goals
- Consolidate extraction logic into `internal/archive/` with exported API
- Add aggregate total bytes limit (500 MB) and file count limit (10,000)
- Port the ampel provider's partial-extraction cleanup to the shared implementation
- Remove provider-local copies and update imports
- Maintain all existing security constraints (path traversal, zip-slip, symlink/hardlink rejection, per-file 100 MB cap, permission model)
- Migrate and consolidate test coverage

### Non-Goals
- Changing the existing per-file size limit (100 MB stays)
- Adding directory depth limits or duplicate entry detection (out of scope, can be added later)
- Changing the complypack content format or the `ResolveComplypackPath` API contract
- Modifying the OpenSCAP provider (it does not extract archives)

## Decisions

### D1: New package at `internal/archive/`

**Decision:** Create `internal/archive/` for the shared extraction code.

**Rationale:** `internal/` is already the established location for shared code in this repo (`internal/version/`). The `archive` name is descriptive and follows Go naming conventions. This package has no provider-specific dependencies — it operates on filesystem paths and io readers only.

**Alternative considered:** `internal/complypack/`. Rejected because the extraction logic is generic tar.gz handling; the "complypack" concern belongs to the callers (`ResolveComplypackPath` knows about "content" sibling directories, which is complypack-specific, but `ExtractTarGz` is general-purpose).

### D2: Export `ResolveComplypackPath` and `ExtractTarGz` as the API; keep constants unexported

**Decision:** Export `ResolveComplypackPath` and `ExtractTarGz` as the package's public API. Keep all three limit constants unexported: `maxExtractedFileSize`, `maxTotalExtractedSize`, `maxExtractedFileCount`. Keep `writeFileFromTar` unexported.

**Rationale:** The constants are implementation details of the extraction function. No caller has a legitimate reason to reference specific limit values — the error messages already include numeric values for diagnostics. Keeping them unexported reduces API surface, avoids accidental coupling, and allows limit values to change without API concern. This aligns with Convention VII (Convention Over Configuration): the package provides opinionated defaults, not configuration points.

### D3: Aggregate counters in the extraction loop

**Decision:** Add `totalBytes int64` and `fileCount int` counters to the `ExtractTarGz` loop. Check `fileCount` before extracting each regular file. Update `totalBytes` after each successful `writeFileFromTar` call (using the returned byte count). This establishes a deterministic check precedence: file count is checked pre-extraction, aggregate bytes post-extraction. When both limits could be exceeded by the same entry, the file count error surfaces first.

The `writeFileFromTar` signature changes from `error` to `(int64, error)` to return actual bytes written. The aggregate counter uses actual bytes written (not tar header size declarations) because headers can lie. If `writeFileFromTar` returns an error, extraction aborts with that error immediately — the aggregate counter is not updated.

**Rationale:** This is the simplest approach with minimal overhead. The counters are checked at each iteration of the extraction loop, so an archive that exceeds limits is rejected as early as possible. Having `writeFileFromTar` return the bytes written enables accurate tracking without re-reading the file. The limit values (500 MB total, 10,000 files) provide ~50x headroom over realistic complypack sizes (typically < 10 MB with < 100 files) while preventing pathological cases.

**Alternative considered:** Pre-scanning tar headers for file sizes before extraction. Rejected because tar headers can lie about file sizes (the actual compressed content may differ), and it would require a second pass.

**Boundary semantics:** Both limits use strict greater-than comparison (`>`). Exactly 500 MB or exactly 10,000 files is allowed; the limit triggers on the value that exceeds it. Directory entries do not count toward the file count (they are structural, not content; the aggregate bytes limit provides sufficient inode protection).

### D4: Include partial-extraction cleanup in shared code

**Decision:** The shared `ResolveComplypackPath` includes `os.RemoveAll(extractDir)` on extraction error, matching the ampel provider's existing behavior.

**Rationale:** The OPA provider's missing cleanup was a bug introduced by the copy-paste. The cleanup prevents the idempotent check from reusing corrupted content on retry. Including it in the shared implementation fixes the OPA provider's behavior.

## Risks / Trade-offs

- **[Structural change]** Moving code to `internal/archive/` changes the project's isolation model where providers were previously self-contained. -> Mitigation: `internal/version/` already establishes the precedent for shared internal packages. The extraction logic is genuinely shared and identical.
- **[Test migration effort]** Tests from `unpack_test.go` and `server_test.go` must be migrated to the new package. -> Mitigation: The ampel provider's test suite is more comprehensive and serves as the primary source. OPA-specific extraction tests that duplicate coverage are removed rather than migrated.
- **[Behavioral change for OPA provider]** OPA provider now cleans up partial extractions, which it previously did not. -> Mitigation: This is a bug fix. Leaving corrupted partial extractions for reuse is incorrect behavior.
- **[Disk space exhaustion]** If the filesystem runs out of space during extraction, `writeFileFromTar` will return an I/O error, triggering the cleanup path. If cleanup itself fails due to disk pressure, the partial extraction directory may persist; the idempotent check will reuse it on retry, which may produce incorrect results. This is an existing limitation not addressed by this change.
