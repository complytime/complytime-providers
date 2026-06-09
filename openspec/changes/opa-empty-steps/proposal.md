## Why

The OPA provider returns `steps: []` in its assessment logs when all checks
pass for a given requirement. The Gemara EvaluationLog CUE schema requires
at least one step per assessment (`steps: [#AssessmentStep, ...#AssessmentStep]`),
so `cue vet` rejects any evaluation log produced by the OPA provider when
requirements have no violations. This breaks schema validation for passing
scans and prevents downstream consumers from distinguishing "all checks
passed" from "no checks were run."

Discovered while validating complytime/complyctl#553 in devpod.

Fixes: https://github.com/complytime/complytime-providers/issues/63

## What Changes

- `ToScanResponse` in `cmd/opa-provider/results/results.go` synthesizes
  passing `AssessmentLog` entries for requirements that had no findings
  (failures/warnings). Each passing assessment includes one `Step` per
  successfully-scanned target with `ResultPassed` and a descriptive message.
- The `reverseMap` parameter (already passed to `ToScanResponse`) is used
  to determine the full set of evaluated requirements. Requirements present
  in the reverse map but absent from the findings groups are treated as
  fully passing.
- Only targets with `Status == "scanned"` contribute passing steps; error
  targets are excluded from synthesized assessments.

## Capabilities

### New Capabilities

### Modified Capabilities

- `opa-scan`: `ToScanResponse` now produces an `AssessmentLog` with at
  least one `Step` for every requirement in the reverse mapping, ensuring
  Gemara schema compliance. Previously, requirements with zero violations
  produced no assessment entry.

### Removed Capabilities

## Impact

- `cmd/opa-provider/results/results.go`: `ToScanResponse` gains a new
  block after finding-based grouping that iterates `reverseMap` to create
  passing assessments for uncovered requirement IDs
- `cmd/opa-provider/results/results_test.go`: New test cases covering
  all-passing, mixed, error-target exclusion, and nil-reverse-map scenarios
- No changes to `server.go`, `scan.go`, or any other package -- the fix
  is fully contained in the results package

## Constitution Alignment

Assessed against the Unbound Force org constitution.

### I. Autonomous Collaboration

**Assessment**: N/A

This change does not affect artifact-based communication patterns. The
output format (provider.ScanResponse) is unchanged; it now contains
additional entries that were previously missing.

### II. Composability First

**Assessment**: PASS

The fix is fully contained within `results.ToScanResponse()` with no new
dependencies. The function's signature is unchanged. The behavior is
additive: existing callers receive strictly more data (passing assessments)
without any breaking changes.

### III. Observable Quality

**Assessment**: PASS

The change directly improves observable quality by ensuring the OPA
provider's output conforms to the Gemara EvaluationLog CUE schema.
Assessment logs now contain machine-parseable passing steps instead of
empty arrays.

### IV. Testability

**Assessment**: PASS

Four new unit tests verify the behavior in isolation: all-passing
requirements, mixed pass/fail, error-target exclusion, and
nil-reverse-map backward compatibility. All tests use the existing
testify framework and require no external dependencies.
