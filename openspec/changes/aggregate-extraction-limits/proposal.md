## Why

PR #52 added secure tar.gz extraction for complypack content in the ampel provider (`cmd/ampel-provider/server/unpack.go`), porting the logic from the OPA provider (`cmd/opa-provider/server/server.go`). Both implementations cap individual extracted files at 100 MB but do not enforce aggregate limits on total extracted content or file count. An archive containing many small files (each under 100 MB) could consume significant disk space or exhaust inodes during extraction.

Additionally, the two extraction implementations are functionally identical copy-pasted code. Applying the aggregate limits separately to each copy would violate the Single Source of Truth principle and create ongoing maintenance risk. Ref: [complytime/complytime-providers#71](https://github.com/complytime/complytime-providers/issues/71).

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

Assessed against the Unbound Force org constitution.

### I. Autonomous Collaboration

**Assessment**: PASS

The shared package produces self-describing errors with clear context (file path, limit exceeded, etc.). No changes to artifact-based communication patterns.

### II. Composability First

**Assessment**: PASS

The `internal/archive/` package is standalone with no provider-specific dependencies. Both providers consume it identically. The package does one thing (secure archive extraction) and does it well.

### III. Observable Quality

**Assessment**: PASS

All limits are defined as named constants with documentation. Error messages include the specific limit value and the entry that triggered the violation, enabling clear diagnostics.

### IV. Testability

**Assessment**: PASS

The shared package is testable in isolation. Tests verify observable side effects (extracted files, error returns, partial cleanup). The dependency injection pattern used by both providers is unchanged.
