# Proposal

## Why

The ampel provider cannot distinguish SKIP from FAIL in ampel's evaluation
output. When a policy bundle contains policies targeting different predicate
types (e.g., branch-rules + check-runs), policies whose declared predicate
types don't match the attestation produce SKIP results that the provider
misreports as failures. This blocks multi-predicate-type policy bundles
including the merge readiness demo (complytime-demos#97). Ampel 1.4.2 now
supports `when:` gate predicates and `--allow-skip`, making the fix possible.

## What Changes

- Add explicit `"SKIP"` case handling in `ParseAmpelOutput` for both
  per-eval-result and overall status (currently falls through to `"fail"`).
- Pass `--allow-skip` flag to the `ampel verify` CLI invocation so root-level
  SKIP is not normalized to FAIL by ampel.
- Add `When *WhenClause` field to the `AmpelPolicy` Go struct so policies
  with `when:` guards can be round-tripped through load/merge/write without
  losing the field.
- Add post-parse business logic in `server.Scan`: if all findings in a
  `PerRepoResult` are SKIP (no policy matched the attestation predicate type),
  convert the result to an error. At least one policy must produce PASS or
  FAIL for a valid evaluation.
- Add test coverage for SKIP handling, the `--allow-skip` flag, the
  `WhenClause` round-trip, and the all-SKIP-to-error conversion.

## Capabilities

### New Capabilities

- `ampel-skip-handling`: Handle ampel SKIP evaluation status in the ampel
  provider, including per-finding SKIP propagation, the `--allow-skip` CLI
  flag, `WhenClause` type support for policy serialization, and all-SKIP
  error detection.

### Modified Capabilities

_(none — no existing spec-level behavior changes)_

## Impact

- **Code**: `cmd/ampel-provider/results/results.go`, `cmd/ampel-provider/scan/scan.go`,
  `cmd/ampel-provider/convert/types.go`, `cmd/ampel-provider/server/server.go`
- **Tests**: `results/results_test.go`, `scan/scan_test.go`, `convert/convert_test.go`,
  `server/server_test.go`
- **Test fixtures**: new `results/testdata/ampel-verify-skip.json`
- **Dependencies**: requires ampel >= 1.4.2 at runtime (CLI tool, not Go module)
- **External**: org-infra branch-protection policies need `when:` guards added
  (separate repo/PR, not in scope of this change)
