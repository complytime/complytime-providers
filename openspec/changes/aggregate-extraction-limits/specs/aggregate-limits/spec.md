## ADDED Requirements

### Requirement: Aggregate extracted bytes limit

The `ExtractTarGz` function MUST track the cumulative bytes written across all files during extraction. The cumulative byte counter MUST only be incremented by bytes written from successfully completed file extractions, based on actual bytes written to disk (not tar header size declarations). When the total is strictly greater than `maxTotalExtractedSize` (500 MB), extraction MUST abort with an error. The aggregate limit is checked after each file write completes; the actual total bytes on disk when the limit triggers may exceed `maxTotalExtractedSize` by up to `maxExtractedFileSize` (100 MB) for the last file. The `writeFileFromTar` helper MUST return the number of bytes written to enable accurate aggregate tracking. Files already written MUST remain on disk (cleanup is the caller's responsibility via `ResolveComplypackPath`).

#### Scenario: Archive total size exceeds aggregate limit
- **GIVEN** a tar.gz archive containing files whose combined size is strictly greater than 500 MB
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction MUST return an error after the cumulative bytes exceed the limit
- **AND** the error message MUST include the aggregate limit value, the current accumulated value, and the name of the tar entry that triggered the violation

#### Scenario: Archive total size at exact limit
- **GIVEN** a tar.gz archive containing files whose combined size equals exactly 500 MB
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction MUST complete successfully (the limit value itself is allowed)

#### Scenario: Archive total size within aggregate limit
- **GIVEN** a tar.gz archive containing files whose combined size is under 500 MB
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction MUST complete successfully

### Requirement: Maximum extracted file count

The `ExtractTarGz` function MUST track the number of regular files extracted. Directory entries do NOT count toward the file count limit (directories are structural, not content; the aggregate bytes limit and per-file size limit provide sufficient protection). The file count is checked before extracting each regular file. When the count would exceed `maxExtractedFileCount` (10,000), extraction MUST abort with an error before extracting the entry that exceeds the limit.

#### Scenario: Archive file count exceeds limit
- **GIVEN** a tar.gz archive containing more than 10,000 regular file entries
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction MUST return an error before extracting the 10,001st file
- **AND** the error message MUST include the file count limit value and the name of the tar entry that triggered the violation

#### Scenario: Archive file count at exact limit
- **GIVEN** a tar.gz archive containing exactly 10,000 regular file entries
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction MUST complete successfully (the limit value itself is allowed)

#### Scenario: Archive file count within limit
- **GIVEN** a tar.gz archive containing fewer than 10,000 regular file entries
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction MUST complete successfully

### Requirement: Check precedence for dual limits

When both limits could be exceeded by the same entry, the file count check takes precedence because it is evaluated before extraction of each file, while the aggregate bytes check is evaluated after extraction.

#### Scenario: Both limits would be exceeded
- **GIVEN** a tar.gz archive that would exceed both the file count and aggregate bytes limits
- **WHEN** `ExtractTarGz` is called
- **THEN** the file count error MUST be returned (not the aggregate bytes error)

### Requirement: Shared extraction package

Tar.gz extraction logic MUST be consolidated into `internal/archive/`. Both the ampel and OPA providers MUST import from this shared package. Provider-local copies of `extractTarGz`, `writeFileFromTar`, and `resolveComplypackPath` MUST be removed.

The shared `ExtractTarGz` function MUST preserve all security constraints from the original implementations: path traversal rejection, zip-slip guard, symlink/hardlink rejection, per-file size cap (100 MB), file mode 0600, directory mode 0750.

#### Scenario: Both providers use the shared package
- **GIVEN** the ampel and OPA provider server packages
- **WHEN** examining their imports
- **THEN** both MUST import `internal/archive` for complypack extraction
- **AND** neither MUST contain local `extractTarGz` or `writeFileFromTar` functions

#### Scenario: Security constraints preserved
- **GIVEN** the shared `ExtractTarGz` function
- **WHEN** processing a tar.gz archive
- **THEN** path traversal, zip-slip, symlink/hardlink rejection, per-file size cap, and permission model MUST behave identically to the original implementations

## MODIFIED Requirements

### Requirement: Partial extraction cleanup on error

Previously: The ampel provider cleaned up partial extractions on error; the OPA provider did not.

The shared `ResolveComplypackPath` function MUST call `os.RemoveAll` on the extraction directory when `ExtractTarGz` returns an error, preventing subsequent calls from reusing incomplete content via the idempotent directory check. This ensures safe retry behavior: after cleanup, a subsequent call will re-attempt extraction rather than reusing corrupted content.

#### Scenario: Failed extraction is cleaned up
- **GIVEN** a tar.gz archive that triggers an extraction error (e.g., oversized file, path traversal, aggregate limit exceeded)
- **WHEN** `ResolveComplypackPath` is called
- **THEN** any partially extracted files MUST be removed
- **AND** the extraction directory MUST not exist after the error

#### Scenario: Retry after failed extraction succeeds
- **GIVEN** a previous call to `ResolveComplypackPath` failed due to an extraction error
- **WHEN** `ResolveComplypackPath` is called again with a valid archive
- **THEN** extraction MUST succeed with correct content (not corrupted leftovers from the prior attempt)

## REMOVED Requirements

_(none)_
