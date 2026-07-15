## ADDED Requirements

### Requirement: Aggregate extracted bytes limit

The `ExtractTarGz` function MUST track the cumulative bytes written across all files during extraction. When the total exceeds `maxTotalExtractedSize` (500 MB), extraction MUST abort with an error describing the limit and the entry that caused it. Files already written MUST remain on disk (cleanup is the caller's responsibility via `ResolveComplypackPath`).

#### Scenario: Archive total size exceeds aggregate limit
- **GIVEN** a tar.gz archive containing files whose combined size exceeds 500 MB
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction SHALL return an error after the cumulative bytes exceed the limit
- **AND** the error message SHALL include the aggregate limit value

#### Scenario: Archive total size within aggregate limit
- **GIVEN** a tar.gz archive containing files whose combined size is under 500 MB
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction SHALL complete successfully

### Requirement: Maximum extracted file count

The `ExtractTarGz` function MUST track the number of regular files extracted. When the count exceeds `maxExtractedFileCount` (10,000), extraction MUST abort with an error describing the limit.

#### Scenario: Archive file count exceeds limit
- **GIVEN** a tar.gz archive containing more than 10,000 regular file entries
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction SHALL return an error before extracting the entry that exceeds the limit
- **AND** the error message SHALL include the file count limit value

#### Scenario: Archive file count within limit
- **GIVEN** a tar.gz archive containing fewer than 10,000 regular file entries
- **WHEN** `ExtractTarGz` is called
- **THEN** extraction SHALL complete successfully

### Requirement: Shared extraction package

Tar.gz extraction logic MUST be consolidated into `internal/archive/`. Both the ampel and OPA providers MUST import from this shared package. Provider-local copies of `extractTarGz`, `writeFileFromTar`, and `resolveComplypackPath` MUST be removed.

#### Scenario: Both providers use the shared package
- **GIVEN** the ampel and OPA provider server packages
- **WHEN** examining their imports
- **THEN** both SHALL import `internal/archive` for complypack extraction
- **AND** neither SHALL contain local `extractTarGz` or `writeFileFromTar` functions

## MODIFIED Requirements

### Requirement: Partial extraction cleanup on error

Previously: The ampel provider cleaned up partial extractions on error; the OPA provider did not.

The shared `ResolveComplypackPath` function MUST call `os.RemoveAll` on the extraction directory when `ExtractTarGz` returns an error, preventing subsequent calls from reusing incomplete content via the idempotent directory check.

#### Scenario: Failed extraction is cleaned up
- **GIVEN** a tar.gz archive that triggers an extraction error (e.g., oversized file, path traversal)
- **WHEN** `ResolveComplypackPath` is called
- **THEN** any partially extracted files SHALL be removed
- **AND** the extraction directory SHALL not exist after the error

## REMOVED Requirements

_(none)_
