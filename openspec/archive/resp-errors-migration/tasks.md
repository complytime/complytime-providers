## 1. Vendor Update (separate commit)

- [x] 1.1 Run `go get github.com/complytime/complyctl@main` to pull the version that includes the `Errors` field on `ScanResponse` (complyctl#510)
- [x] 1.2 Run `make vendor` to update vendored files
- [x] 1.3 Verify `ScanResponse.Errors` field is present in vendored `pkg/provider/client.go`
- [x] 1.4 Commit the vendor update separately: `vendor: update complyctl to pick up ScanResponse.Errors (complyctl#510)`

## 2. OPA Provider Migration

- [x] 2.1 In `cmd/opa-provider/results/results.go`, remove the TODO comment (lines 171-173) referencing complyctl#510
- [x] 2.2 In `cmd/opa-provider/results/results.go` `ToScanResponse`, replace the `"scan-error"` assessment group logic (lines 210-224) with code that appends operational errors to `resp.Errors`
- [x] 2.3 Remove `const errorReqID = "scan-error"` and associated grouping logic from `ToScanResponse`
- [x] 2.4 Update `ScanStatusAssessment` signature to remove the `writeErr` parameter: `func ScanStatusAssessment(targetResults []*PerTargetResult) provider.AssessmentLog`
- [x] 2.5 Remove the `"result-persistence"` step and write-error message logic from `ScanStatusAssessment`
- [x] 2.6 In `cmd/opa-provider/server/server.go`, move write-error reporting to `resp.Errors` at the call site instead of passing it to `ScanStatusAssessment`
- [x] 2.7 Update `cmd/opa-provider/results/results_test.go` `TestToScanResponse_ErrorTargets` to assert errors appear in `resp.Errors` and not in assessments
- [x] 2.8 Update `cmd/opa-provider/results/results_test.go` `ScanStatusAssessment` tests to remove `writeErr` parameter and `"result-persistence"` step assertions

## 3. Verification

- [x] 3.1 Run `make test` and confirm all tests pass
- [x] 3.2 Run `make lint` and confirm no lint errors
- [x] 3.3 Grep `cmd/opa-provider/` for `"scan-error"` and confirm zero matches in Go source files
- [x] 3.4 Run `make build` and confirm all provider binaries build successfully

## 4. Follow-up

- [x] 4.1 File a follow-up issue to migrate the Ampel provider's `"scan-error"` pattern to `resp.Errors`
