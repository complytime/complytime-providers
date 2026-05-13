# ADR-0002: Error Propagation and Target Variable Constants

**Status**: Accepted
**Date**: 2026-05-13
**Decision makers**: jpower432 (reviewer), Fabian Ortiz

## Context

During continued review of PR #12, two refinements to the OPA provider scan pipeline were identified:

1. **WritePerTargetResult errors are silently discarded.** `WritePerTargetResult()` returns an error, but all three call sites in `server.go` (`processRemoteBranches`, `processLocalInput` via `evalAndParse`) log the error and discard it. ADR-0001 accepted this as "log-and-continue" behavior, but on further review this prevents the caller (`complyctl`) from knowing that result persistence failed. Combined with the scan-level status assessment added in ADR-0001, the caller currently receives a "pass" or "fail" status with no indication that the underlying result files may be incomplete.

2. **Target variable keys are hardcoded strings.** Keys like `"url"`, `"input_path"`, `"branch"`, `"access_token"`, `"scan_path"`, and `"opa_bundle_ref"` appear as string literals throughout `server.go`, `loader/loader.go`, and `scan/scan.go`. A typo in any one site would silently produce incorrect behavior (empty string from map lookup).

## Decision

### Error Propagation via errors.Join

Revise the error handling strategy from ADR-0001. Instead of log-and-continue for `WritePerTargetResult` errors, propagate them through the call chain:

- `processRemoteBranches` signature changes from `[]*results.PerTargetResult` to `([]*results.PerTargetResult, error)` — collects write errors instead of logging them.
- `processLocalInput` signature changes the same way.
- `evalAndParse` already returns `error` — stop swallowing the `WritePerTargetResult` error and return it.
- `processTarget` signature changes from `[]*results.PerTargetResult` to `([]*results.PerTargetResult, error)`.
- `Scan()` aggregates all returned errors with `errors.Join` and returns the combined error alongside the response.

This means `Scan()` can return both a non-nil response (with assessment results) and a non-nil error (indicating persistence failures). The caller receives complete scan results and knows whether result files were written successfully.

### Target Variable Constants

Extract hardcoded target variable key strings into named constants in a central location, eliminating the risk of silent typo-induced bugs across `server.go`, `loader/loader.go`, and validation code.

## Alternatives Considered

### Alternative 1: Keep log-and-continue (ADR-0001 status quo)

Maintain the current behavior where `WritePerTargetResult` errors are logged and discarded. The scan-level status assessment already provides a summary to the caller.

Rejected because the caller has no way to detect that result files are incomplete or missing. In compliance scanning, a scan that reports "pass" but fails to persist evidence is worse than a scan that reports an error — the user believes they have evidence when they don't.

### Alternative 2: Treat write failures as fatal

Fail the entire target or scan immediately when `WritePerTargetResult` returns an error. This is the simplest model — any write failure stops processing.

Rejected because it sacrifices scan completeness for strictness. If writing results for target 3 of 10 fails (e.g., transient disk issue), the remaining 7 targets would not be scanned. Returning both results and an aggregated error preserves completeness while surfacing the failure.

## Consequences

### Positive

- **Caller visibility**: `complyctl` can detect result persistence failures and report them to the user, rather than silently missing evidence.
- **Scan completeness preserved**: Returning results alongside errors means all targets are still scanned even when some writes fail.
- **Typo prevention**: Named constants for target variable keys are checked at compile time (unused constant is a build error), preventing silent bugs from string literal typos.

### Negative

- **Dual return semantics**: `Scan()` returning both a response and an error requires callers to handle both, which is less common than the typical "nil response on error" Go pattern. The `complyctl` plugin framework must handle this correctly.
- **Signature churn**: Four internal functions gain an error return, changing their call sites in `server.go`.

### Risks

- **complyctl gRPC adapter limitation (VERIFIED)**: The `complyctl` gRPC adapter at `pkg/provider/server.go` discards the `ScanResponse` when `Scan()` returns a non-nil error:
  ```go
  resp, err := s.impl.Scan(ctx, &ScanRequest{Targets: targets})
  if err != nil {
      return nil, err  // response is DISCARDED
  }
  ```
  This is also standard gRPC behavior — client-side generated code returns nil for the response when a non-nil error status is received. Therefore, the dual return `(response, error)` pattern originally proposed cannot surface both results and write failures to the caller. **Mitigation**: Instead of returning write errors as `Scan()`'s error, embed them in the `ScanResponse` via the `scan-status` `AssessmentLog` by extending `ScanStatusAssessment()` to accept and report write failures as a `result-persistence` step. `Scan()` always returns `(response, nil)` for completed scans.
- This decision should be revisited if `complyctl` introduces a dedicated mechanism for reporting partial failures or persistence status.

## Related Decisions

- [ADR-0001: OPA Provider Scan Architecture Redesign](0001-opa-provider-scan-architecture-redesign.md) — amends the error handling section, which originally specified log-and-continue for `WritePerTargetResult`
