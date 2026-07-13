## ADDED Requirements

### Requirement: Generate persists matched requirement IDs

The ampel provider's `Generate` RPC SHALL persist the full set of matched requirement IDs to a `scan-config.json` file in the generated directory so they are available during `Scan`.

#### Scenario: Requirement IDs written during Generate

- **WHEN** the `Generate` RPC matches one or more requirement IDs against granular AMPEL policies
- **THEN** a `scan-config.json` file SHALL be written to the generated directory containing all matched requirement IDs and a generation timestamp

#### Scenario: No matched requirements skips scan config

- **WHEN** the `Generate` RPC matches zero requirement IDs (no policies found)
- **THEN** no `scan-config.json` file SHALL be written and `Generate` SHALL return success with no policy output

### Requirement: Scan reads persisted requirement IDs

The ampel provider's `Scan` RPC SHALL read the `scan-config.json` file written by `Generate` and pass the requirement IDs to the results processing function.

#### Scenario: Scan config present

- **WHEN** `Scan` is invoked and `scan-config.json` exists in the generated directory
- **THEN** the requirement IDs SHALL be read from the file and passed to `ToScanResponse`

#### Scenario: Scan config missing

- **WHEN** `Scan` is invoked and `scan-config.json` does not exist in the generated directory
- **THEN** `Scan` SHALL log a warning and fall back to findings-only behavior (no synthetic passing assessments)

### Requirement: Scan config round-trip fidelity

The `WriteScanConfig` and `ReadScanConfig` functions SHALL produce identical `ScanConfig` values for a round-trip write-then-read operation.

#### Scenario: Write then read preserves data

- **WHEN** a `ScanConfig` with requirement IDs `["BP-1.01", "BP-2.03"]` is written and then read back
- **THEN** the read `ScanConfig` SHALL contain the same requirement IDs in the same order
