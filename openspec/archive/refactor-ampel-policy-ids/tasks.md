## 1. Testdata Policy Files

- [x] [P] 1.1 Create `cmd/ampel-provider/convert/testdata/policies/require-pull-request.json` with `id` set to `require-pull-request` and `meta.controls[0].id` set to `pull-request-enforcement` (CEL/predicates/messages from current `BP-01.01-require-pull-request.json`)
- [x] [P] 1.2 Create `cmd/ampel-provider/convert/testdata/policies/minimum-approvals.json` with `id` set to `minimum-approvals` and `meta.controls[0].id` set to `approval-requirements`
- [x] [P] 1.3 Create `cmd/ampel-provider/convert/testdata/policies/block-force-push.json` with `id` set to `block-force-push` and `meta.controls[0].id` set to `force-push-restriction`
- [x] [P] 1.4 Create `cmd/ampel-provider/convert/testdata/policies/prevent-admin-bypass.json` with `id` set to `prevent-admin-bypass` and `meta.controls[0].id` set to `admin-bypass-prevention`
- [x] [P] 1.5 Create `cmd/ampel-provider/convert/testdata/policies/require-code-owner-review.json` with `id` set to `require-code-owner-review` and `meta.controls[0].id` set to `code-owner-enforcement`
- [x] 1.6 Delete old `cmd/ampel-provider/convert/testdata/policies/BP-*.json` files (all 5)

## 2. Assessment Plan Fixtures

- [x] [P] 2.1 Update `cmd/ampel-provider/convert/testdata/assessment-plan-full.json`: replace `BP-1.01` .. `BP-5.01` requirement IDs with `require-pull-request`, `minimum-approvals`, `block-force-push`, `prevent-admin-bypass`, `require-code-owner-review`
- [x] [P] 2.2 Update `cmd/ampel-provider/convert/testdata/assessment-plan-subset.json`: replace `BP-1.01` and `BP-3.01` with `require-pull-request` and `block-force-push`

## 3. Expected Bundle Fixtures

- [x] [P] 3.1 Regenerate `cmd/ampel-provider/convert/testdata/ampel-bundle-expected-full.json`: update all policy `id` fields and `meta.controls[].id` references to semantic IDs (CEL/predicates/messages unchanged)
- [x] [P] 3.2 Regenerate `cmd/ampel-provider/convert/testdata/ampel-bundle-expected-subset.json`: update policy IDs and control references to semantic IDs; fix pre-existing inconsistency by using the current CEL expressions, dual predicate URLs, and description text from the actual source policies

## 4. Test Code ID Updates

- [x] 4.1 Update `cmd/ampel-provider/convert/convert_test.go`: replace all hardcoded `BP-X.YY` ID string literals with the corresponding semantic IDs in: `TestLoadGranularPolicies` (expectedIDs array), `TestMatchPolicies_Subset` (assertions), `TestMatchPolicies_UnmatchedRule` (input and assertion), `TestMatchPolicies_DuplicateRequirements` (input), and `TestMergeToBundle` (inline test data and assertions). No test logic or structure changes beyond ID string updates.

## 5. Recursive LoadGranularPolicies

- [x] 5.1 Modify `cmd/ampel-provider/convert/convert.go`: replace `os.ReadDir` + flat loop in `LoadGranularPolicies` with `filepath.WalkDir` to recursively walk subdirectories; skip symlinks (directory symlinks not followed, file symlinks skipped); add duplicate ID detection (return error naming both paths); preserve existing skip logic (non-JSON, PolicyFileName)
- [x] 5.2 Add test for subdirectory loading in `cmd/ampel-provider/convert/convert_test.go`: create a test that places policy files in a subdirectory and verifies `LoadGranularPolicies` finds them
- [x] 5.3 Add test for mixed flat and subdirectory loading: verify policies at root and in subdirectories are all loaded
- [x] 5.4 Add test for PolicyFileName skip in subdirectory: verify `complytime-ampel-policy.json` is skipped even when found in a subdirectory
- [x] 5.5 Add test for nested subdirectories (2+ levels deep): verify policy files at multiple nesting depths are all loaded
- [x] 5.6 Add test for non-JSON files in subdirectories: verify non-JSON files are skipped
- [x] 5.7 Add test for malformed JSON in subdirectory: verify `LoadGranularPolicies` returns an error
- [x] 5.8 Add test for empty ID in subdirectory file: verify `LoadGranularPolicies` returns an error
- [x] 5.9 Add test for unreadable file in subdirectory: verify `LoadGranularPolicies` returns an error (use `os.Chmod` to simulate)
- [x] 5.10 Add test for duplicate policy IDs across subdirectories: verify `LoadGranularPolicies` returns an error naming both paths
- [x] 5.11 Add test for symlink to directory: verify the symlink target is not walked
- [x] 5.12 Add test for empty subdirectory: verify no error and other policies still load

## 6. Validation

- [x] 6.1 Run `make test` and verify all tests pass
- [x] 6.2 Run `make lint` and verify no lint issues
- [x] 6.3 Run `make build` and verify binaries build successfully
- [x] 6.4 Assess documentation impact: add CHANGELOG.md entry for semantic ID refactoring and recursive policy loading

<!-- spec-review: passed -->
<!-- code-review: passed -->
