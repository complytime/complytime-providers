## Why

The upstream complyctl project resolved the `RequirementID` naming confusion
(complyctl#622) via PRs #624 and #632 (merged Jun 26-29, 2026). The fix did
not rename `RequirementID` to `PlanID` as this issue originally suggested.
Instead, complyctl separated the two concepts:

- `AssessmentConfiguration.PlanID` now holds the Gemara assessment plan `id`
- `AssessmentConfiguration.RequirementID` now holds the Gemara `requirement-id`
- A new `MatchID()` method was added for complyctl-internal routing (prefers
  `PlanID`, falls back to `RequirementID`)

The vendored complyctl (`v1.0.0-beta.0`) already contains both fields and
`MatchID()`. However, the providers in this repository still have doc strings
and comments written under the pre-#632 assumption that `RequirementID`
held the plan ID. Test fixtures only populate `RequirementID`, not both
fields. READMEs conflate the two concepts.

A separate upstream PR (#640, in review) adds `PlanID` to `AssessmentLog`
on the response side with a backward-compatible fallback, so this work can
proceed in parallel.

## What Changes

- Fix misleading doc strings on `MappingEntry` in
  `cmd/opa-provider/generate/mapping.go` that say "Gemara assessment plan
  RequirementID" when the field correctly holds Gemara requirement IDs
- Fix misleading doc strings and comments across all three providers that
  conflate plan IDs with requirement IDs
- Update test fixtures to populate both `PlanID` and `RequirementID` on
  `provider.AssessmentConfiguration` structs, reflecting the post-#632
  upstream contract
- Clarify ID semantics in provider READMEs

## Capabilities

### New Capabilities

_(none)_

### Modified Capabilities

- `opa-generate`: Doc strings on `MappingEntry`, `MatchRequirements`,
  `ResolveRequirementID`, and `extractRequirementID` are corrected to
  accurately describe the ID semantics
- `ampel-generate`: Doc string on `MatchPolicies` is corrected
- `openscap-generate`: Doc comments in tailoring code are corrected

## Impact

- `cmd/opa-provider/generate/mapping.go`: Doc strings on `MappingEntry`
  struct and `MatchRequirements` function
- `cmd/opa-provider/results/results.go`: Doc strings on
  `ResolveRequirementID`, `extractRequirementID`, `ToScanResponse`,
  and the `reqGroup` internal type
- `cmd/opa-provider/server/server.go`: Doc comment on `Generate`
- `cmd/ampel-provider/convert/convert.go`: Doc string on `MatchPolicies`
- `cmd/ampel-provider/results/results.go`: Doc comments on
  `ToScanResponse` and `reqGroup`
- `cmd/openscap-provider/xccdf/tailoring.go`: No doc changes needed
  (comments reference "rule" correctly)
- Test files across all three providers: `AssessmentConfiguration` struct
  literals updated to populate both `PlanID` and `RequirementID`
- `cmd/opa-provider/README.md`: Clarify `requirement_id` field semantics
  in mapping file documentation
- `cmd/ampel-provider/README.md`: Clarify requirement ID semantics
- `cmd/openscap-provider/README.md`: Clarify `RequirementID` semantics
- **No behavioral changes**: All matching logic remains identical; providers
  continue to use `cfg.RequirementID` directly, which is correct
- **No breaking changes**: No struct renames, no JSON field changes, no
  wire format changes
- Fixes: https://github.com/complytime/complytime-providers/issues/94
