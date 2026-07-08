## Context

The Ampel provider converts `ampel verify` attestation results into
`provider.ScanResponse` via two functions in
`cmd/ampel-provider/results/results.go`:

1. `ParseAmpelOutput` — parses the in-toto attestation into
   `PerRepoResult` with `[]Finding`.
2. `ToScanResponse` — groups findings by requirement ID into
   `[]provider.AssessmentLog`.

Currently, `ParseAmpelOutput` extracts `error.message` and
`assessment.message` into `Finding.Reason` but discards
`error.guidance`. `ToScanResponse` ignores the per-step messages
entirely and always sets `AssessmentLog.Message` to a generic
`"%d of %d repositories passed"` format string.

The OPA provider already implements the desired pattern: it uses the
first non-passing step's message as the `AssessmentLog.Message`. All
org-infra Ampel policies (7 tenets across 5 policies) already provide
quality `error.message`, `error.guidance`, and `assessment.message`
content.

complyctl's evaluator at `internal/output/evaluator.go:203` already
maps `AssessmentLog.Recommendation` to `gemara.AssessmentLog`, so no
complyctl changes are required.

## Goals / Non-Goals

**Goals:**

- Surface tenet-level failure messages as `AssessmentLog.Message` so
  evaluation logs and reports show actionable policy-defined text.
- Propagate `error.guidance` to `AssessmentLog.Recommendation` for
  platform-specific remediation instructions in reports.
- Follow the OPA provider's failure-case message pattern and extend
  it for passing results by surfacing `assessment.message`.
- Fix vocabulary from "repositories" to "checks" in the fallback
  count string.

**Non-Goals:**

- Modifying the Ampel attestation format or policy schema.
- Changing complyctl's evaluator or report formatting logic.
- Changing the OPA provider (already implements the desired pattern).
- Adding multi-step guidance aggregation (only the first non-passing
  step's guidance is used, matching the message override pattern).

## Decisions

### D1: Use OPA-style message override in ToScanResponse

**Decision**: After building all steps for a requirement group,
iterate through steps to find the first non-passing step. Use its
`Message` as `AssessmentLog.Message`. If all steps pass, use the
first step's `Message`. Fall back to a count string only when all
step messages are empty.

**Rationale**: For non-passing results this matches the OPA
provider's existing pattern. For all-passing results this improves
on OPA's pattern by using the tenet's `assessment.message` instead
of a generic count string, since Ampel tenets consistently provide
descriptive pass messages. The fallback count string uses "checks"
(not "targets" as in OPA) because Ampel evaluates tenet checks per
repository. The first non-passing step's message is chosen because
it represents the most actionable finding — users need to know what
failed and why.

**Alternatives considered**:
- Concatenate all non-passing messages: Produces verbose, hard-to-read
  output. The step-level detail is already available in the evaluation
  log's `steps` array for users who need the full picture.
- Keep the count string and rely on complyctl's override: complyctl
  already overrides for non-passing results via `matchingStepMessage`,
  but the provider should be accurate at its own layer. Relying on
  downstream correction is fragile and leaves passing results with
  a misleading count message.

### D2: Add Guidance field to Finding struct

**Decision**: Add a `Guidance string` field to the `Finding` struct
and populate it from `er.Error.Guidance` during `ParseAmpelOutput`.
Map it to `AssessmentLog.Recommendation` in `ToScanResponse` using
the same first-non-passing-step pattern as the message.

**Rationale**: `Finding` is the intermediate representation between
parsing and response construction. Adding a field here keeps the data
flow clean: `ParseAmpelOutput` is responsible for extraction,
`ToScanResponse` is responsible for mapping. The `Recommendation`
field on `provider.AssessmentLog` exists for exactly this purpose.

**Alternatives considered**:
- Store guidance in the `Reason` field alongside the message
  (concatenated): Mixes two semantically different pieces of
  information. `Reason` maps to `Step.Message` which complyctl uses
  for the description; guidance belongs in `Recommendation`.

### D3: First non-passing step for both Message and Recommendation

**Decision**: Use the same step (first non-passing) to source both
`Message` and `Recommendation`. This keeps the two fields contextually
coherent — the recommendation explains how to fix the issue described
in the message.

**Rationale**: A mismatch between message and recommendation (e.g.,
message about approvals but guidance about force push) would confuse
users. Using the same source step ensures coherence.

## Risks / Trade-offs

- **[Risk] Empty step messages from future Ampel versions**:
  If future Ampel versions produce attestations without
  `assessment.message` or `error.message`, the message override
  would produce empty strings.
  -> Mitigation: The fallback count string
  (`"X of Y checks passed"`) covers this case.

- **[Trade-off] Only first non-passing message surfaced**:
  When multiple steps fail for different reasons, only the first
  failure's message appears in `AssessmentLog.Message`. Other
  failures are still visible in the step-level detail.
  -> Acceptable: matches the OPA provider pattern and keeps the
  top-level message concise. Full detail is in the steps array.

- **[Risk] Test fixture updates**: Existing tests assert the generic
  count string format. All assertions in `results_test.go` that
  check `Message` will need updating.
  -> Mitigation: Straightforward — test data already includes
  `Reason` fields that will become the expected messages.
