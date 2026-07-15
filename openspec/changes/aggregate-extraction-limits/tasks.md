## 1. Create shared internal/archive/ package

- [ ] 1.1 Create `internal/archive/archive.go` with `// SPDX-License-Identifier: Apache-2.0` header, exported `ResolveComplypackPath(contentPath string) (string, error)`, `ExtractTarGz(archive, dst string) error`, and unexported `writeFileFromTar(path string, r io.Reader) (int64, error)` (returns bytes written)
- [ ] 1.2 Add unexported constants: `maxExtractedFileSize = 100 << 20` (100 MB), `maxTotalExtractedSize = 500 << 20` (500 MB), `maxExtractedFileCount = 10_000`
- [ ] 1.3 Add aggregate `totalBytes` counter in `ExtractTarGz` loop, updated after each successful `writeFileFromTar` call; abort with descriptive error (including limit value, current accumulated value, and entry name) when total is strictly greater than `maxTotalExtractedSize`
- [ ] 1.4 Add `fileCount` counter in `ExtractTarGz` loop, checked before each regular file extraction (directories do not count); abort with descriptive error (including limit value and entry name) when count would exceed `maxExtractedFileCount`. File count check precedes byte count check, establishing deterministic error precedence
- [ ] 1.5 Include `os.RemoveAll(extractDir)` cleanup in `ResolveComplypackPath` on extraction error (ported from ampel provider)
- [ ] 1.6 Preserve all existing security checks: path traversal, zip-slip, symlink/hardlink rejection, per-file size cap, file mode 0600, directory mode 0750

## 2. Create shared package tests

- [ ] 2.1 Create `internal/archive/archive_test.go` with `// SPDX-License-Identifier: Apache-2.0` header and test helpers (reuse ampel's `createTarGz` + `tarEntry` pattern)
- [ ] 2.2 Migrate existing tests (maintaining equivalent or deeper assertion coverage): directory passthrough, tar.gz extraction, idempotent skip, path traversal rejection, absolute path rejection, symlink rejection, hard link rejection, oversized file rejection, non-existent path, corrupt archive, dot directory entry, file permissions
- [ ] 2.3 Add `TestExtractTarGz_AggregateSizeExceeded` — use a small number of files that push the aggregate over the limit (e.g., 6 files x ~90 MB each) rather than creating a full 500 MB archive; assert error message includes limit value and entry name
- [ ] 2.4 Add `TestExtractTarGz_FileCountExceeded` — use zero-length regular file entries to minimize I/O while triggering the counter; guard with `testing.Short()` if execution exceeds ~2 seconds; assert error message includes limit value and entry name
- [ ] 2.5 Add `TestExtractTarGz_AggregateSizeAtExactLimit` — archive with total bytes == `maxTotalExtractedSize` succeeds (boundary: limit value itself is allowed)
- [ ] 2.6 Add `TestExtractTarGz_FileCountAtExactLimit` — archive with exactly `maxExtractedFileCount` files succeeds (boundary: limit value itself is allowed)
- [ ] 2.7 Add `TestResolveComplypackPath_FailedExtractionCleansUp` — verify partial extraction is removed on error, then call again with a valid archive and verify it extracts successfully (regression test for OPA cleanup bug per TC-006)
- [ ] 2.8 Add `TestExtractTarGz_AggregateSizeExceededByOne` — archive with total bytes == `maxTotalExtractedSize + 1` fails (boundary: one byte over the limit)
- [ ] 2.9 Add `TestExtractTarGz_FileCountExceededByOne` — archive with `maxExtractedFileCount + 1` files fails (boundary: one file over the limit)
- [ ] 2.10 Add `TestExtractTarGz_BothLimitsExceeded` — archive that exceeds both file count and aggregate bytes limits; assert file count error is returned, not the aggregate bytes error (check precedence per spec)

## 3. Migrate ampel provider

- [ ] 3.1 Remove `cmd/ampel-provider/server/unpack.go`
- [ ] 3.2 Update `cmd/ampel-provider/server/server.go` to import `internal/archive` and call `archive.ResolveComplypackPath` instead of the local function
- [ ] 3.3 Remove `cmd/ampel-provider/server/unpack_test.go` (covered by shared tests)
- [ ] 3.4 Update any remaining ampel server tests that reference extraction functions to use the shared package

## 4. Migrate OPA provider

- [ ] 4.1 Remove `resolveComplypackPath`, `extractTarGz`, `writeFileFromTar`, and `maxExtractedFileSize` from `cmd/opa-provider/server/server.go`
- [ ] 4.2 Update `cmd/opa-provider/server/server.go` to import `internal/archive` and call `archive.ResolveComplypackPath` instead of the local function
- [ ] 4.3 Remove extraction-specific tests from `cmd/opa-provider/server/server_test.go` (covered by shared tests)
- [ ] 4.4 Update any remaining OPA server tests that reference extraction functions to use the shared package

## 5. Verification

- [ ] 5.1 Run `make test` and confirm all existing and new tests pass
- [ ] 5.2 Run `make lint` and confirm no lint errors
- [ ] 5.3 Run `make build` and confirm all three provider binaries build successfully
- [ ] 5.4 Grep for `extractTarGz` and `writeFileFromTar` in `cmd/` — confirm zero matches in Go source files (only in `internal/archive/`)
- [ ] 5.5 Grep for `maxExtractedFileSize` in `cmd/` — confirm zero matches

## 6. Documentation

- [ ] 6.1 Add CHANGELOG.md entry for: aggregate extraction limits, shared archive package, OPA cleanup fix
- [ ] 6.2 Update AGENTS.md project structure tree to include `internal/archive/` under the internal/ section, and update the Architecture section prose to reflect that `internal/archive/` is now shared between providers alongside `internal/version/`
- [ ] 6.3 Update README.md project structure tree to include `internal/archive/`, and update the architecture description to reflect that `internal/archive/` provides shared extraction utilities between providers
<!-- spec-review: passed -->
