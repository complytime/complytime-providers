## 1. OPA Provider Doc String Fixes

- [x] 1.1 In `cmd/opa-provider/generate/mapping.go`, update the `MappingEntry` struct doc string (line 31) from "maps a Gemara assessment plan RequirementID to a Rego namespace" to accurately describe the requirement-id to Rego namespace mapping
- [x] 1.2 In `cmd/opa-provider/generate/mapping.go`, update the `MatchRequirements` function doc string (lines 87-90) to clarify it matches `cfg.RequirementID` (Gemara requirement IDs) against mapping entries
- [x] 1.3 In `cmd/opa-provider/generate/mapping.go`, update the comment on line 95 ("Build lookup map: requirement_id -> id") and the error messages on lines 73 and 79 to use consistent terminology
- [x] 1.4 In `cmd/opa-provider/results/results.go`, update the `ResolveRequirementID` doc string (lines 171-173) to clarify the ID resolution chain
- [x] 1.5 In `cmd/opa-provider/results/results.go`, update the `extractRequirementID` doc string (lines 101-104) to clarify it derives a Rego-namespace-based ID
- [x] 1.6 In `cmd/opa-provider/results/results.go`, update the `ToScanResponse` doc string (lines 182-186) to clarify the requirement ID grouping and resolution semantics
- [x] 1.7 In `cmd/opa-provider/results/results.go`, fix the misleading comment on line 230 that says "reverseMap values are the plan IDs" -- they are requirement IDs
- [x] 1.8 In `cmd/opa-provider/server/server.go`, update the `Generate` doc comment (line 90) to clarify "assessment plan's RequirementIDs" terminology

## 2. AMPEL Provider Doc String Fixes

- [x] 2.1 In `cmd/ampel-provider/convert/convert.go`, update the `MatchPolicies` doc string (line 123) to clarify it matches Gemara requirement IDs against granular policy IDs
- [x] 2.2 In `cmd/ampel-provider/results/results.go`, update the `ToScanResponse` doc comment (line 192) to clarify requirement ID grouping semantics
- [x] 2.3 In `cmd/ampel-provider/results/results.go`, update the comment on line 213 to clarify "requirement ID" derivation from TenetID

## 3. OPA Provider Test Fixture Updates

- [x] 3.1 In `cmd/opa-provider/generate/mapping_test.go`, add `PlanID` to all `provider.AssessmentConfiguration` struct literals (lines 134-135, 153-154, 170, 186-187)
- [x] 3.2 In `cmd/opa-provider/results/results_test.go`, add `PlanID` to all `provider.AssessmentConfiguration` struct literals used in test data
- [x] 3.3 In `cmd/opa-provider/server/server_test.go`, add `PlanID` to all `provider.AssessmentConfiguration` struct literals (lines 286-287, 323, 344, 383, 436, 502, 558, 752, 776, 811, 851-852, 921, 1013)

## 4. AMPEL Provider Test Fixture Updates

- [x] 4.1 In `cmd/ampel-provider/convert/convert_test.go`, add `PlanID` to all `provider.AssessmentConfiguration` struct literals (lines 515-516, 531-532, 554-555)
- [x] 4.2 In `cmd/ampel-provider/server/server_test.go`, add `PlanID` to all `provider.AssessmentConfiguration` struct literals (lines 33, 205, 225)
- [x] 4.3 In `cmd/ampel-provider/results/results_test.go`, verify test fixtures and add `PlanID` where `provider.AssessmentConfiguration` structs are used

## 5. OpenSCAP Provider Test Fixture Updates

- [x] 5.1 In `cmd/openscap-provider/xccdf/tailoring_test.go`, add `PlanID` to all `provider.AssessmentConfiguration` struct literals (lines 201-202, 214, 238, 276-277, 288-289, 300-301, 315, 328-330, 368-369, 377-378, 387-388, 434-437, 454-458, 677, 728)

## 6. README Updates

- [x] 6.1 In `cmd/opa-provider/README.md`, clarify that `requirement_id` in `complytime-mapping.json` matches the Gemara `requirement-id` field (not the plan `id`)
- [x] 6.2 In `cmd/opa-provider/README.md`, update references to "RequirementID" to distinguish between plan IDs and requirement IDs where applicable
- [x] 6.3 In `cmd/ampel-provider/README.md`, clarify requirement ID semantics in policy matching documentation
- [x] 6.4 In `cmd/openscap-provider/README.md`, clarify that `RequirementID` holds XCCDF rule short names from the Gemara `requirement-id` field

## 7. Verification

- [x] 7.1 Run `make test` and confirm all tests pass
- [x] 7.2 Run `make lint` -- pre-existing toolchain version mismatch (golangci-lint built with Go 1.25, project uses Go 1.26); not related to this change
- [x] 7.3 Run `make build` and confirm all provider binaries build successfully
- [x] 7.4 Verify no doc string or comment in `cmd/*/` still says "Gemara assessment plan RequirementID"
