## Context

The OPA provider's `ToScanResponse` function groups conftest findings by
requirement ID into `AssessmentLog` entries. When all checks pass for a
requirement, conftest produces zero findings, so no `AssessmentLog` is
created. The resulting `steps: []` violates the Gemara `EvaluationLog`
CUE schema which requires `steps: [#AssessmentStep, ...#AssessmentStep]`
(at least one step per assessment).

The `reverseMap` parameter already maps Rego policy namespaces to Gemara
requirement IDs, giving us the full set of evaluated requirements. By
comparing this set against the findings-derived groups, we can identify
requirements that passed all checks.

## Goals / Non-Goals

### Goals
- Ensure every requirement in the `reverseMap` produces an `AssessmentLog`
  with at least one `Step`, satisfying the Gemara CUE schema
- Distinguish "all checks passed" from "no checks were run" by populating
  steps with explicit passing entries per scanned target
- Maintain deterministic output ordering for stable test assertions

### Non-Goals
- Changing the `ToScanResponse` function signature or the `provider.Step`
  type
- Adding passing-step synthesis to the ampel or openscap providers (they
  don't have this gap -- openscap always has a rule result per assessment,
  ampel omits assessments for requirements with no findings entirely)
- Modifying `ScanStatusAssessment` (it already populates steps for every
  target)

## Decisions

**Synthesize in `ToScanResponse` after the findings loop.** The fix adds
a second pass over `reverseMap` values after the existing findings-based
grouping. For each Gemara requirement ID not already in the `groups` map,
a new `reqGroup` is created with one `Step` per target whose
`Status == "scanned"`. This keeps the fix localized to a single block and
avoids touching the findings-processing logic.

**Use `sortedValues` helper for deterministic iteration.** Go maps have
non-deterministic iteration order. A `sortedValues` helper extracts unique
values from the reverse map and sorts them, ensuring stable output across
runs. This aligns with the existing `sort.Strings(order)` pattern used
for the findings-based groups.

**Exclude error targets from synthesized steps.** Targets with
`Status == "error"` had operational failures (network timeouts, git clone
errors) and should not appear as "passed" in synthesized assessments. Only
targets that were successfully scanned contribute passing steps. This
parallels how `ScanStatusAssessment` treats error targets.

**Guard on `reverseMap != nil`.** When no reverse map is provided (e.g.,
during direct conftest invocation without a mapping file), the synthesizer
is skipped entirely. This preserves backward compatibility for callers
that don't use requirement mapping.

## Risks / Trade-offs

**Synthesized steps may mask scan gaps.** If a Rego policy file fails to
load or is misconfigured, conftest may silently skip it. The synthesizer
would then create a passing assessment for the mapped requirement despite
the policy never having been evaluated. This is an existing limitation of
the conftest-based approach and is partially mitigated by the
`ScanStatusAssessment` reporting target-level errors. Accepted as
out-of-scope for this change.

**Additional allocations for passing assessments.** Each passing
requirement creates N steps (one per scanned target). For large target
sets this adds memory. In practice, the number of targets is small
(typically < 50), and the allocations are negligible compared to the
conftest invocations themselves. Accepted.
