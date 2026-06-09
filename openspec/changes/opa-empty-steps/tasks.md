## 1. Synthesize Passing Assessments in ToScanResponse

- [ ] 1.1 In `ToScanResponse` (`cmd/opa-provider/results/results.go`), after the existing finding-based grouping loop, add a block that iterates `reverseMap` to identify Gemara requirement IDs not already present in the `groups` map. For each uncovered requirement, create a `reqGroup` with one `provider.Step` per successfully-scanned target (`Status == "scanned"`) using `ResultPassed` and message `"all checks passed"`. Append the new requirement IDs to the `order` slice so they are included in the sorted, deduplicated output.

## 2. Add Unit Tests

- [ ] 2.1 Add `TestToScanResponse_PassingAssessmentsForNoFindings` (`cmd/opa-provider/results/results_test.go`): targets with no findings and a populated reverse map produce one passing `AssessmentLog` per mapped requirement, each with one step per scanned target
- [ ] 2.2 Add `TestToScanResponse_MixedFindingsAndPassingRequirements` (`cmd/opa-provider/results/results_test.go`): when some requirements have findings and others do not, both failing and passing assessments appear in the response
- [ ] 2.3 Add `TestToScanResponse_PassingAssessmentsExcludeErrorTargets` (`cmd/opa-provider/results/results_test.go`): targets with `Status == "error"` do not contribute steps to synthesized passing assessments
- [ ] 2.4 Add `TestToScanResponse_NoPassingAssessmentsWithoutReverseMap` (`cmd/opa-provider/results/results_test.go`): when `reverseMap` is nil, no passing assessments are synthesized (backward compatibility)

## 3. Verification

- [ ] 3.1 Run `make test` and `make lint` to confirm all tests pass and no lint issues
