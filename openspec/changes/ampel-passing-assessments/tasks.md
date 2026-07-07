## 1. Scan Config Persistence

- [x] 1.1 Create `cmd/ampel-provider/generate/scanconfig.go` with `ScanConfig` struct (`RequirementIDs []string`, `GeneratedAt string`), `WriteScanConfig(dir string, requirementIDs []string) error`, and `ReadScanConfig(dir string) (*ScanConfig, error)` functions
- [x] 1.2 Create `cmd/ampel-provider/generate/scanconfig_test.go` with tests for write/read round-trip fidelity, empty requirement IDs, and missing file error handling
- [x] 1.3 Add `ScanConfigPath` helper to `cmd/ampel-provider/config/config.go` returning the path to `scan-config.json` in the generated directory

## 2. Generate Integration

- [x] 2.1 Update `Generate` method in `cmd/ampel-provider/server/server.go` to collect matched requirement IDs from `convert.MatchPolicies` result and call `generate.WriteScanConfig` after writing the policy bundle
- [x] 2.2 Add or update test in `cmd/ampel-provider/server/server_test.go` to verify that `Generate` writes `scan-config.json` with the correct requirement IDs

## 3. Passing Assessment Synthesis

- [x] 3.1 Add `buildSyntheticSteps(repoResults []*PerRepoResult) []provider.Step` helper to `cmd/ampel-provider/results/results.go` that creates one passing step per non-error repo result using `repoDisplayName@branch` naming
- [x] 3.2 Update `ToScanResponse` signature in `cmd/ampel-provider/results/results.go` to accept `allRequirementIDs []string` and synthesize passing assessments for requirement IDs not present in the findings-derived groups map
- [x] 3.3 Add tests in `cmd/ampel-provider/results/results_test.go` for: requirement with zero findings gets passing assessment, nil requirement ID list falls back to findings-only, error repos excluded from synthetic steps, mixed findings and passing requirements

## 4. Scan Integration

- [x] 4.1 Update `Scan` method in `cmd/ampel-provider/server/server.go` to read `scan-config.json` via `generate.ReadScanConfig`, log warning if missing, and pass requirement IDs to `results.ToScanResponse`
- [x] 4.2 Add or update test in `cmd/ampel-provider/server/server_test.go` to verify that `Scan` passes requirement IDs through to `ToScanResponse` and handles missing scan config gracefully

## 5. Verification

- [x] 5.1 Run `make test` and confirm all tests pass
- [x] 5.2 Run `make lint` and confirm no lint violations
- [x] 5.3 Run `make build` and confirm all provider binaries build successfully
