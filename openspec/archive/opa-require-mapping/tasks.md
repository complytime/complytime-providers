## 1. LoadMapping Error Differentiation

- [x] 1.1 Add `os.IsNotExist` check in `LoadMapping` (`cmd/opa-provider/generate/mapping.go`) to return a distinguishable error for missing files vs. malformed/invalid files
- [x] 1.2 Add unit tests for the new error differentiation: missing file returns a file-not-found error, malformed JSON returns a parse error, invalid fields return a validation error

## 2. Generate Method: Replace Fallback with Error

- [x] 2.1 Replace the fallback branch in `Generate` (`cmd/opa-provider/server/server.go` lines 133-143) to return `{Success: false, ErrorMessage: "..."}` when `LoadMapping` reports a missing file, with a message naming `complytime-mapping.json` and its expected location
- [x] 2.2 Ensure `Generate` returns `{Success: false}` with the original validation error when `LoadMapping` reports a malformed or invalid mapping file
- [x] 2.3 Update `TestGenerate_WithoutMapping` (`cmd/opa-provider/server/server_test.go`) to assert `Success: false` and verify the error message contains the mapping file name

## 3. Scan: Remove Fallback Path

- [x] 3.1 Remove the `scanCfg.IDs == nil` fallback branch in `evalAndParse` (`cmd/opa-provider/server/server.go` line 418) so nil IDs are treated as a configuration error
- [x] 3.2 Remove `EvalPolicy` and `constructConftestTestCommand` from `cmd/opa-provider/scan/scan.go` (the `--all-namespaces` functions) since they have no remaining callers
- [x] 3.3 Remove the nil-map passthrough in `ResolveRequirementID` (`cmd/opa-provider/results/results.go`) since reverse mapping is now always present

## 4. Test Updates

- [x] 4.1 Update `TestGenerate_Scan_FallbackPath` (`cmd/opa-provider/server/server_test.go`) to verify the error response instead of fallback behavior
- [x] 4.2 Remove or update `TestWriteAndReadScanConfig_NullIDs` (`cmd/opa-provider/generate/scanconfig_test.go`) and `TestResolveRequirementID_NilMapping` (`cmd/opa-provider/results/results_test.go`) since nil-IDs and nil-map paths no longer exist

## 5. Verification

- [x] 5.1 Run `make test` and `make lint` to confirm all tests pass and no lint issues
