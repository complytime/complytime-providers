# Tasks: OPA Generate Implementation

## Generate Package

- [x] **T1: Create `generate` package with mapping types** -- Add `cmd/opa-provider/generate/mapping.go` with `MappingFile`, `MappingEntry` structs, `LoadMapping(bundleDir string) (*MappingFile, error)`, and `MatchRequirements(configs []provider.AssessmentConfiguration, mapping *MappingFile) (namespaces []string, reverseMap map[string]string, warnings []string)`. Add `cmd/opa-provider/generate/mapping_test.go` with tests for: valid mapping, missing file, empty file, malformed JSON, duplicate requirement IDs, empty fields. Covers REQ-GEN-003, REQ-GEN-004, REQ-GEN-005, REQ-MAP-001, REQ-MAP-002, REQ-MAP-003.

- [x] **T2: Create scan config types and I/O** -- Add `cmd/opa-provider/generate/scanconfig.go` with `ScanConfig` struct (`IDs []string`, `ReverseMapping map[string]string`, `BundleDir string`, `GeneratedAt string`), `WriteScanConfig(dir string, ids []string, reverseMap map[string]string, bundleDir string) error`, and `ReadScanConfig(dir string) (*ScanConfig, error)`. Add `cmd/opa-provider/generate/scanconfig_test.go` with tests for: write/read round-trip, missing file returns error, malformed file returns error, null IDs round-trip. Covers REQ-GEN-006, REQ-GEN-007.

## Config Changes

- [x] **T3: Add `GeneratedDirPath` to config** -- Add `GeneratedDir = "generated"` constant and `GeneratedDirPath() string` method to `cmd/opa-provider/config/config.go`. Add `GeneratedDirPath` to the directories list in `EnsureDirectories`. Add test in `config_test.go` verifying the path and directory creation.

## Server Changes

- [x] **T4: Implement `Generate` method** -- Replace the stub in `cmd/opa-provider/server/server.go` with the full implementation: validate `req.Configuration` is non-empty (REQ-GEN-008), check tools (REQ-GEN-009), ensure directories, extract `opa_bundle_ref` from merged global/target variables (REQ-GEN-010), pull bundle via `scan.PullBundle` (REQ-GEN-002), load mapping via `generate.LoadMapping` (REQ-GEN-003), match requirements (REQ-GEN-004), write scan config (REQ-GEN-006). On missing mapping file, write fallback scan config with nil values (REQ-GEN-007). Add `mergeVariables` helper.

- [x] **T5: Update Scan to read scan config** -- Modify `processTarget` in `server.go` to read `scan-config.json` via `generate.ReadScanConfig` before evaluation. If present with non-null namespaces, use `scan.EvalPolicyWithNamespaces` (REQ-SCAN-001); otherwise fall back to `scan.EvalPolicy` (REQ-SCAN-002). Pass reverse mapping through to results.

- [x] **T6: Wire reverse mapping through Scan** -- Update the call chain from `processTarget` → `evalAndParse` → results aggregation to pass the optional `reverseMap` from scan config to `ToScanResponse`. When scan config is absent or has no mapping, pass `nil` to preserve current behavior (REQ-COMPAT-001).

## Scan Package Changes

- [x] **T7: Add namespace-filtered conftest command** -- Add `EvalPolicyWithNamespaces(inputPath, policyDir string, namespaces []string, runner CommandRunner) ([]byte, error)` and `constructConftestTestCommandWithNamespaces(inputPath, policyDir string, namespaces []string) (string, []string)` to `cmd/opa-provider/scan/scan.go`. Add tests in `scan_test.go` verifying `--namespace` flags are constructed correctly and `--all-namespaces` is not present. Covers REQ-SCAN-001.

## Results Package Changes

- [x] **T8: Add reverse mapping to results** -- Add `ResolveRequirementID(derivedID string, reverseMap map[string]string) string` to `cmd/opa-provider/results/results.go`. Update `ToScanResponse` signature to accept an optional `reverseMap map[string]string` parameter. When non-nil, resolve Rego-derived IDs to Gemara IDs before grouping (REQ-SCAN-003). When nil, use derived IDs unchanged (REQ-SCAN-004). Add tests for: mapping hit, mapping miss (returns derived ID unchanged), nil mapping. Covers REQ-COMPAT-002.

## Server Tests

- [x] **T9: Add Generate server tests** -- Add tests to `cmd/opa-provider/server/server_test.go` for: Generate with valid mapping (happy path), Generate without mapping file (fallback), Generate with empty configuration (returns error), Generate with missing `opa_bundle_ref` (returns error), Generate with tool check failure (returns error), Generate with bundle pull failure (returns error), Generate with partial requirement matches (some matched, some warned).

- [x] **T10: Add integration-level fallback test** -- Add a test that exercises the full Generate → Scan path with no mapping file to verify the scan response is identical to the current stub-Generate behavior. Covers REQ-COMPAT-001.

## Verification

- [x] **T11: Run `make test` and `make lint`** -- Verify all existing and new tests pass with no regressions. Verify no lint violations. Fix any issues found.
