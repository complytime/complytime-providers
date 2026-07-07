## Why

The Ampel provider's `AssessmentLog.Message` field always uses a generic
count string (`"X of Y repositories passed"`) regardless of the actual
tenet evaluation outcome. This obscures the actionable failure messages
that Ampel policies already define (`error.message` and `error.guidance`)
and produces misleading descriptions in complyctl evaluation logs and
reports. The word "repositories" is also semantically inaccurate since
the count represents findings per requirement, not repositories scanned.

Meanwhile, the `error.guidance` field — which contains platform-specific
remediation instructions — is parsed from the Ampel attestation but
silently discarded, never reaching the `Recommendation` field on
`provider.AssessmentLog`.

All 7 tenets across the 5 production policies in org-infra already
provide quality `error.message` and `error.guidance` content. The data
is available; it is just not surfaced.

## What Changes

- The top-level `AssessmentLog.Message` will use the tenet's
  `error.message` (on failure) or `assessment.message` (on pass) instead
  of the generic count string, aligning with the OPA provider's existing
  behavior.
- The `error.guidance` field from Ampel tenet evaluations will be
  propagated to `AssessmentLog.Recommendation`, giving users
  platform-specific remediation instructions in complyctl reports.
- The `Finding` struct will gain a `Guidance` field so `ParseAmpelOutput`
  can carry guidance through to `ToScanResponse`.

## Capabilities

### New Capabilities

- `actionable-scan-message`: Surface tenet-level pass/fail messages as
  the `AssessmentLog.Message` instead of a generic count string.
- `guidance-recommendation`: Propagate the Ampel `error.guidance` field
  to `AssessmentLog.Recommendation` for remediation context in reports.

## Impact

- `cmd/ampel-provider/results/results.go`: `Finding` struct gains
  `Guidance` field; `ParseAmpelOutput` extracts `error.guidance`;
  `ToScanResponse` builds `Message` from tenet content and populates
  `Recommendation`.
- `cmd/ampel-provider/results/results_test.go`: Updated assertions for
  new `Message` and `Recommendation` values.
- No complyctl changes required — complyctl already handles
  `Recommendation` in `providerToGemaraAssessment()`.
- No policy changes required — all org-infra policies already provide
  the needed fields.
- No proto/API changes — `provider.AssessmentLog` already has
  `Recommendation` and `Message` fields.
- `CHANGELOG.md`: Entry needed for the user-visible message format
  change and new `Recommendation` population.
- ResultSet-level `error.guidance` is out of scope — it applies to
  operational errors routed to `resp.Errors`, which is a string
  slice without structured guidance support.
