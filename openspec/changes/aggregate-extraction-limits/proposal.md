## Why

PR #52 added secure tar.gz extraction for complypack content in the ampel provider (`cmd/ampel-provider/server/unpack.go`), porting the logic from the OPA provider (`cmd/opa-provider/server/server.go`). Both implementations cap individual extracted files at 100 MB but do not enforce aggregate limits on total extracted content or file count. An archive containing many small files (each under 100 MB) could consume significant disk space or exhaust inodes during extraction.

Additionally, the two extraction implementations are functionally identical copy-pasted code. Applying the aggregate limits separately to each copy would violate the Single Source of Truth principle (Constitution I) and create ongoing maintenance risk — as already evidenced by the OPA provider's missing partial-extraction cleanup, a bug introduced by the original copy-paste. This consolidation was explicitly deferred as a non-goal in the `ampel-complypack-content` OpenSpec (now archived): "Extracting shared tar extraction code into a common internal package (can be done later once a third provider needs it)." The aggregate limits feature provides the forcing function to consolidate now. Ref: [complytime/complytime-providers#71](https://github.com/complytime/complytime-providers/issues/71).

### Scope Justification

This OpenSpec bundles three concerns that are inseparable:

1. **Aggregate extraction limits** — the feature requested by issue #71
2. **Code consolidation** into `internal/archive/` — required by Constitution Principle I (Single Source of Truth); duplicating the aggregate limit fix across both providers would guarantee future drift
3. **OPA provider cleanup bug fix** — a direct consequence of the duplication; the shared implementation inherits the ampel provider's correct cleanup behavior

## What Changes

- Extract the shared tar.gz extraction logic (`resolveComplypackPath`, `extractTarGz`, `writeFileFromTar`) into a new `internal/archive/` package, eliminating the duplicate implementations in both providers
- Add aggregate extraction limits: 500 MB total extracted bytes and 10,000 maximum file count
- Port the ampel provider's partial-extraction cleanup (`os.RemoveAll` on error) into the shared implementation, fixing the OPA provider's missing cleanup
- Update both providers to import from `internal/archive/` instead of using package-local copies
- Add comprehensive tests for aggregate limits in the shared package

## Capabilities

### New Capabilities
- `aggregate-extraction-limits`: Extraction aborts when total extracted bytes exceed 500 MB or file count exceeds 10,000

### Modified Capabilities
- `tar-extraction`: Moved from provider-local implementations to shared `internal/archive/` package with consistent behavior (including partial-extraction cleanup)

### Removed Capabilities
- `provider-local-extraction`: The per-provider `extractTarGz`, `writeFileFromTar`, and `resolveComplypackPath` functions are removed in favor of the shared package

## Impact

- **New package**: `internal/archive/` — shared extraction utilities with all security constraints
- **Removed file**: `cmd/ampel-provider/server/unpack.go` — extraction logic moves to `internal/archive/`
- **Modified file**: `cmd/opa-provider/server/server.go` — extraction logic removed, imports `internal/archive/`
- **Modified file**: `cmd/ampel-provider/server/server.go` — imports `internal/archive/` instead of local functions
- **Test migration**: `cmd/ampel-provider/server/unpack_test.go` tests move to `internal/archive/archive_test.go`; OPA extraction tests in `cmd/opa-provider/server/server_test.go` are removed (covered by shared tests)
- **Behavioral change**: OPA provider now cleans up partial extractions on error (previously it did not)

## Constitution Alignment

Assessed against the ComplyTime Constitution (`.specify/memory/constitution.md`).

### I. Single Source of Truth (Centralized Constants)

**Assessment**: PASS — primary driver

This change directly addresses a Single Source of Truth violation: identical extraction code duplicated across two providers. Consolidation into `internal/archive/` ensures limit constants, security checks, and cleanup behavior are defined once.

### II. Simplicity & Isolation

**Assessment**: PASS

The `internal/archive/` package is standalone with no provider-specific dependencies. Both providers consume it identically. Functions follow the Single Responsibility Principle.

### III. Incremental Improvement

**Assessment**: PASS — scope justified

While this change bundles three concerns (aggregate limits, consolidation, OPA cleanup fix), all three are inseparable: applying limits without consolidation would violate Principle I, and the cleanup fix is a direct consequence of the duplication. See Scope Justification above.

### IV. Readability First

**Assessment**: PASS

The shared package uses explicit naming (`maxTotalExtractedSize`, `maxExtractedFileCount`) and self-documenting error messages that include limit values and the entry that triggered the violation.

### V. Do Not Reinvent the Wheel

**Assessment**: N/A

No new dependencies introduced. Uses Go stdlib (`archive/tar`, `compress/gzip`, `io`).

### VI. Composability (The Unix Philosophy)

**Assessment**: PASS

The `internal/archive/` package does one thing (secure archive extraction) and does it well. `ExtractTarGz` and `ResolveComplypackPath` are independently usable.

### VII. Convention Over Configuration

**Assessment**: PASS

Limits are opinionated defaults (500 MB, 10,000 files) kept as unexported constants. Users do not need to configure them.
