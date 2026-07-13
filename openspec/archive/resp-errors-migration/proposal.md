## Why

The OPA provider currently reports operational errors (failed clones, bundle-pull
failures, write errors) as synthetic `"scan-error"` assessment entries inside
`ScanResponse.Assessments`. This conflates infrastructure failures with actual
compliance findings, making it harder for consumers to distinguish "the scan
could not run" from "the scan ran and found a violation." Now that complyctl#510
has merged and `ScanResponse.Errors` is available, the OPA provider should use
the dedicated error channel instead of polluting the assessment stream. This
addresses Issue #21. The Ampel provider has the same pattern and will be
migrated in a separate follow-up issue.

## What Changes

- Update the vendored `complyctl` dependency to pick up the `ScanResponse.Errors` field from PR #510.
- Modify `ToScanResponse` in `cmd/opa-provider/results/results.go` to place
  operational errors (targets with `Status == "error"` and no findings) into
  `resp.Errors` instead of creating synthetic `"scan-error"` assessment entries.
- Migrate the OPA provider's `ScanStatusAssessment` write-error reporting to
  also use `resp.Errors` where appropriate, keeping the `"scan-status"`
  assessment for scan-health summarization only.
- Remove the `const errorReqID = "scan-error"` sentinel and all associated
  grouping logic from the OPA provider.
- Remove the TODO comment in `cmd/opa-provider/results/results.go` (lines 171-173).
- Update OPA provider tests to assert on `resp.Errors` instead of
  `"scan-error"` assessment entries.

## Capabilities

### New Capabilities

- `resp-errors`: Migration of operational error reporting from synthetic
  assessment entries to the `ScanResponse.Errors` field in the OPA provider.

### Modified Capabilities

_(none -- no existing spec-level requirements are changing)_

## Impact

- **Code**: `cmd/opa-provider/results/results.go`, `cmd/opa-provider/results/results_test.go`,
  `cmd/opa-provider/server/server.go` (write-error propagation).
- **Dependencies**: Requires vendoring an updated `complyctl` that includes
  the `Errors` field on `ScanResponse` (complyctl#510).
- **API surface**: The gRPC `ScanResponse` message will now carry an `errors`
  field. Consumers that previously looked for `RequirementID == "scan-error"`
  assessments must read `resp.Errors` instead. This is a **behavioral change**
  but not a wire-breaking change since the proto field is additive.
- **Other providers**: The OpenSCAP provider does not use synthetic
  `"scan-error"` assessments and is unaffected. The Ampel provider has the
  same pattern but will be migrated in a separate follow-up issue.
