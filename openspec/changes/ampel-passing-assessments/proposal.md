## Why

The ampel provider's `ToScanResponse` only creates `AssessmentLog` entries from findings. When all checks pass for a requirement (zero findings), that requirement is silently absent from the `ScanResponse`. The Gemara EvaluationLog CUE schema requires at least one step per assessment, and complyctl cannot correlate results for requirements that were evaluated but produced no output. This is tracked in [issue #65](https://github.com/complytime/complytime-providers/issues/65).

## What Changes

- Persist the full set of evaluated requirement IDs during `Generate` to a `scan-config.json` file, following the pattern established by the OPA provider.
- Accept the persisted requirement IDs in `ToScanResponse` and synthesize passing `AssessmentLog` entries (with at least one passing `Step`) for requirements that had zero findings.
- Add a `ScanConfig` data structure and `WriteScanConfig`/`ReadScanConfig` helpers to the ampel provider's config or a new `scanconfig` package.
- Update the `Scan` method in `server.go` to read the scan config and pass requirement IDs through to `ToScanResponse`.

## Capabilities

### New Capabilities

- `ampel-scan-config`: Persist and retrieve the set of requirement IDs matched during Generate so they are available during Scan.
- `ampel-passing-assessments`: Synthesize passing assessment logs for requirements with zero findings so every evaluated requirement appears in the scan response.

### Modified Capabilities

_(none)_

## Impact

- **Code**: `cmd/ampel-provider/results/results.go` (signature change to `ToScanResponse`), `cmd/ampel-provider/server/server.go` (Generate persists config, Scan reads it), new scan config helpers.
- **Tests**: New and updated tests in `results/results_test.go` and `server/server_test.go`, plus scan config unit tests.
- **APIs**: `ToScanResponse` gains a new parameter (the set of all requirement IDs). This is an internal API change with no external impact.
- **Dependencies**: None. Uses only stdlib and existing project dependencies.
