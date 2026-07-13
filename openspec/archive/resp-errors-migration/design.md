## Context

The OPA provider reports operational errors (failed git clones, bundle-pull
failures, result-write errors) by inserting synthetic assessment entries with
`RequirementID = "scan-error"` into `ScanResponse.Assessments`. This conflates
infrastructure failures with compliance findings. The OPA provider also has a
`ScanStatusAssessment` function that produces a `"scan-status"` assessment
summarizing scan health and write errors.

With complyctl#510 merged, `ScanResponse` now carries a dedicated `Errors`
field for operational errors. The vendored `complyctl` dependency must be
updated, and the OPA provider's `ToScanResponse` function must migrate from
the synthetic assessment pattern to `resp.Errors`. The Ampel provider has the
same pattern but will be migrated in a separate follow-up issue.

Affected files:

| Provider | File | Concern |
|----------|------|---------|
| OPA | `cmd/opa-provider/results/results.go` | `ToScanResponse`, `ScanStatusAssessment` |
| OPA | `cmd/opa-provider/results/results_test.go` | Tests for error targets and scan-status |
| OPA | `cmd/opa-provider/server/server.go` | Write-error propagation |

The OpenSCAP and Ampel providers are not in scope for this change.

## Goals / Non-Goals

**Goals:**

- Migrate operational error reporting from synthetic `"scan-error"` assessments
  to `ScanResponse.Errors` in the OPA provider.
- Remove the `const errorReqID = "scan-error"` sentinel and associated
  grouping logic from the OPA provider's `ToScanResponse` function.
- Migrate write-error reporting in the OPA provider's `ScanStatusAssessment`
  to use `resp.Errors` instead of a synthetic `"result-persistence"` step.
- Update all tests to assert on `resp.Errors` instead of `"scan-error"`
  assessment entries.
- Remove the TODO comment in `cmd/opa-provider/results/results.go`.

**Non-Goals:**

- Changing the `"scan-status"` assessment itself -- it will continue to
  summarize scan health (success/failure counts per target). Only the
  write-error step moves to `resp.Errors`.
- Modifying the OpenSCAP or Ampel providers (Ampel will be a follow-up issue).
- Changing any provider's `Scan()` RPC return error -- the gRPC-level error
  remains for fatal failures that prevent any response.

## Decisions

### D1: Populate `resp.Errors` inline in `ToScanResponse`

**Decision:** Collect operational errors into `resp.Errors` directly inside
`ToScanResponse`, replacing the `"scan-error"` assessment group logic.

**Rationale:** The error collection is already happening in `ToScanResponse`
where target results are iterated. Keeping it there avoids splitting the
concern across multiple call sites. The function already has access to the
error details it needs.

**Alternative considered:** A separate `CollectErrors` function called by
the server before `ToScanResponse`. Rejected because it would duplicate the
iteration over target results and create two functions that must stay in sync.

### D2: Move write-error reporting out of `ScanStatusAssessment`

**Decision:** The `ScanStatusAssessment` function will no longer accept a
`writeErr` parameter. Write errors will be appended to `resp.Errors` at
the call site in `server.go`.

**Rationale:** Write errors are operational errors, not assessment results.
The `"result-persistence"` step in `ScanStatusAssessment` was a workaround
for the absence of `resp.Errors`. Now that it exists, write errors belong
there. The `ScanStatusAssessment` function becomes purely about summarizing
target scan outcomes.

**Alternative considered:** Keeping `writeErr` in `ScanStatusAssessment` and
duplicating it into `resp.Errors`. Rejected because it would report the same
error in two places.

### D3: Error string format

**Decision:** Use the same error message strings currently in the
`provider.Step.Message` fields (e.g., the target error string, write error
string). No reformatting.

**Rationale:** The messages are already human-readable and contain the
relevant context. Changing them would break any downstream consumers that
parse error messages.

## Risks / Trade-offs

- **[Behavioral change for consumers]** Consumers that previously scanned
  `Assessments` for `RequirementID == "scan-error"` will no longer find
  those entries. **Mitigation:** This is the intended change. The `Errors`
  field is the correct channel. Document in CHANGELOG.md.

- **[Vendor update risk]** Updating the vendored `complyctl` may pull in
  other changes beyond the `Errors` field. **Mitigation:** Review the
  vendor diff carefully. Pin to the specific version that includes #510.

- **[Test coverage gap]** If new `resp.Errors` field semantics differ from
  expectations. **Mitigation:** Write tests that assert both the presence
  of errors in `resp.Errors` and the absence of `"scan-error"` assessments.
