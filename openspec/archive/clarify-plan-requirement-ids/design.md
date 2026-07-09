## Context

The upstream complyctl project (PR #632, merged Jun 29) separated two
previously conflated concepts in `AssessmentConfiguration`:

| Field | Holds | Example (OpenSCAP) | Example (OPA) | Example (AMPEL) |
|-------|-------|--------------------|---------------|-----------------|
| `PlanID` | Gemara `AssessmentPlan.Id` | `"openscap-cramfs-check"` | `"opa-k8s-root-check"` | `"test-plan"` |
| `RequirementID` | Gemara `AssessmentPlan.RequirementId` | `"kernel_module_cramfs_disabled"` | `"CIS-K8S-5.2.6"` | `"require-pull-request"` |

All three providers in this repository match against `cfg.RequirementID`
during Generate. This is correct behavior -- each provider's matching
target is a requirement-level identifier:

- **OpenSCAP**: XCCDF rule short names in the SSG datastream
- **AMPEL**: Granular policy file IDs
- **OPA**: `complytime-mapping.json` `requirement_id` entries

The vendored complyctl (`v1.0.0-beta.0`) already contains both fields and
the `MatchID()` method. However, the provider code has doc strings, comments,
and test fixtures written under the pre-#632 assumption where
`RequirementID` held the plan ID.

Affected files:

| Provider | File | Concern |
|----------|------|---------|
| OPA | `cmd/opa-provider/generate/mapping.go` | `MappingEntry` doc string, `MatchRequirements` doc string |
| OPA | `cmd/opa-provider/results/results.go` | `ResolveRequirementID`, `extractRequirementID`, `ToScanResponse` doc strings |
| OPA | `cmd/opa-provider/server/server.go` | `Generate` doc comment |
| OPA | `cmd/opa-provider/generate/mapping_test.go` | Test fixtures missing `PlanID` |
| OPA | `cmd/opa-provider/results/results_test.go` | Test fixtures missing `PlanID` |
| OPA | `cmd/opa-provider/server/server_test.go` | Test fixtures missing `PlanID` |
| OPA | `cmd/opa-provider/README.md` | ID semantics documentation |
| AMPEL | `cmd/ampel-provider/convert/convert.go` | `MatchPolicies` doc string |
| AMPEL | `cmd/ampel-provider/results/results.go` | `ToScanResponse` doc comment |
| AMPEL | `cmd/ampel-provider/convert/convert_test.go` | Test fixtures missing `PlanID` |
| AMPEL | `cmd/ampel-provider/server/server_test.go` | Test fixtures missing `PlanID` |
| AMPEL | `cmd/ampel-provider/results/results_test.go` | Test fixtures missing `PlanID` |
| AMPEL | `cmd/ampel-provider/README.md` | ID semantics documentation |
| OpenSCAP | `cmd/openscap-provider/xccdf/tailoring_test.go` | Test fixtures missing `PlanID` |
| OpenSCAP | `cmd/openscap-provider/README.md` | ID semantics documentation |

## Goals / Non-Goals

**Goals:**

- Correct all misleading doc strings that conflate plan IDs with
  requirement IDs across all three providers.
- Update test fixtures to populate both `PlanID` and `RequirementID`
  on `provider.AssessmentConfiguration` struct literals, reflecting the
  post-#632 upstream contract where both fields carry distinct values.
- Clarify ID semantics in provider READMEs so users writing
  `complytime-mapping.json` files or Gemara assessment plans understand
  which ID goes where.

**Non-Goals:**

- Renaming `MappingEntry.RequirementID` or the `complytime-mapping.json`
  `requirement_id` field -- these are correctly named.
- Adopting `cfg.MatchID()` in provider matching logic -- providers
  correctly use `cfg.RequirementID` directly, and `MatchID()` is designed
  for complyctl-internal complypack routing, not provider-side content
  matching.
- Adopting `AssessmentLog.PlanID` on the response side -- this depends
  on upstream PR #640 and is a separate follow-up.
- Changing any runtime behavior -- all matching logic remains identical.

## Decisions

### D1: No rename of MappingEntry.RequirementID

**Decision:** Keep `MappingEntry.RequirementID` and the JSON tag
`"requirement_id"` unchanged.

**Rationale:** The field correctly holds Gemara requirement IDs (e.g.,
`CIS-K8S-5.2.6`), not plan IDs. The original issue suggested renaming
to `PlanID` / `plan_id`, but this was based on the pre-#632 understanding
where complyctl was incorrectly putting the plan ID into `RequirementID`.
After #632, `RequirementID` correctly holds the requirement ID, making
the rename unnecessary and incorrect.

**Alternative considered:** Rename to `plan_id` as the issue suggested.
Rejected because the upstream fix changed the semantics: `RequirementID`
now genuinely holds requirement IDs.

### D2: No adoption of MatchID() for provider matching

**Decision:** Providers continue to use `cfg.RequirementID` directly
for matching during Generate.

**Rationale:** `MatchID()` returns `PlanID` when set (e.g.,
`"openscap-cramfs-check"`), falling back to `RequirementID`. All three
providers match against requirement-level identifiers (XCCDF rule short
names, AMPEL policy slugs, OPA mapping `requirement_id` entries). Using
`MatchID()` would return the plan's own identity instead of the matching
key, breaking all provider matching logic.

`MatchID()` is designed for complyctl-internal use: complypack routing
(`buildReqToComplypackRef`) and `resolveAssessmentIDs()` on the results
side. It is not intended for provider-side content matching.

### D3: Test fixtures populate both PlanID and RequirementID

**Decision:** Update test `AssessmentConfiguration` struct literals to
include both `PlanID` and `RequirementID` with distinct values.

**Rationale:** The vendored complyctl now populates both fields with
different values. Tests that only set `RequirementID` do not exercise
the realistic data shape and could mask bugs where code accidentally
reads `PlanID` instead of `RequirementID`.

**Example:**
```go
// Before:
{RequirementID: "CIS-1"}

// After:
{PlanID: "ap-cis-1", RequirementID: "CIS-1"}
```

### D4: Defer AssessmentLog.PlanID adoption

**Decision:** Do not adopt `AssessmentLog.PlanID` on the scan response
side in this change.

**Rationale:** Upstream PR #640 (adding `PlanID` to `AssessmentLog`)
is still in review. The current provider behavior (setting
`AssessmentLog.RequirementID` with the match ID) works correctly thanks
to the backward-compatible fallback in complyctl's `server.go`. A
follow-up change can adopt `AssessmentLog.PlanID` after #640 merges.

## Risks / Trade-offs

- **[Test churn]** Updating test fixtures across all three providers
  produces a large diff with many mechanical struct literal changes.
  **Mitigation:** Changes are purely additive (adding `PlanID` field)
  and can be verified by `make test`.

- **[Upstream drift]** If upstream PR #640 changes the `AssessmentLog`
  contract before this change merges, vendor updates may conflict.
  **Mitigation:** This change does not touch `AssessmentLog` fields,
  so #640 is orthogonal.
