## Why

The ampel provider's test fixtures use opaque, benchmark-coupled IDs (`BP-1.01` .. `BP-5.01`) that are not self-documenting. As part of a coordinated cross-repository refactoring, these IDs are being replaced with semantic, benchmark-agnostic slugs. The canonical granular policies are being centralized in `org-infra/compliance/ampel/` with new IDs, and the Gemara catalog/policy in `complytime-policies` is being updated to match. This change aligns the complytime-providers testdata with those upstream changes and adds support for recursive policy directory loading.

**Connected changes:**
- **org-infra** (`opsx/refactor-ampel-policy-ids`): Restructures `compliance/ampel/` directory, renames policy IDs, updates workflow to source policies from org-infra instead of complytime-providers testdata, deletes old monolithic bundle, updates spec docs.
- **complytime-policies** (GitHub issue): Updates Gemara catalog control/requirement IDs and policy assessment plan IDs to match the new semantic scheme.

## What Changes

- **BREAKING**: Rename all testdata granular policy IDs from `BP-1.01` .. `BP-5.01` to semantic slugs (`require-pull-request`, `minimum-approvals`, `block-force-push`, `prevent-admin-bypass`, `require-code-owner-review`).
- Rename testdata policy filenames to match (e.g., `BP-01.01-require-pull-request.json` becomes `require-pull-request.json`).
- Update `meta.controls[].id` references from `BP-1` .. `BP-5` to semantic control IDs (`pull-request-enforcement`, `approval-requirements`, `force-push-restriction`, `admin-bypass-prevention`, `code-owner-enforcement`).
- Update assessment plan test fixtures (`assessment-plan-full.json`, `assessment-plan-subset.json`) with new requirement IDs.
- Regenerate expected bundle fixtures (`ampel-bundle-expected-full.json`, `ampel-bundle-expected-subset.json`) with new IDs; fix the pre-existing inconsistency where the subset expected bundle has older CEL expressions and predicate URLs that don't match the actual source policies.
- Add recursive directory walking to `LoadGranularPolicies` (using `filepath.WalkDir` instead of `os.ReadDir`) to support structured policy source directories without requiring flat staging.

## Capabilities

### New Capabilities

- `semantic-policy-ids`: Refactoring of ampel testdata policy IDs, filenames, and test fixtures to use self-documenting, benchmark-agnostic identifiers aligned with upstream changes in org-infra and complytime-policies.
- `recursive-policy-loading`: Enhancement to `LoadGranularPolicies` to walk subdirectories, enabling structured policy source directories (e.g., `branch-protection/`, `signed-commits/`) without requiring flat staging.

### Modified Capabilities

## Impact

- **Test fixtures**: All files under `cmd/ampel-provider/convert/testdata/` are updated. Go source code changes: `LoadGranularPolicies` in `convert/convert.go` (recursive walking), and `convert_test.go` (hardcoded `BP-*` ID string literals updated to semantic IDs -- no test logic changes).
- **CI**: `make test` must pass after all fixture updates. No workflow changes in this repo.
- **Upstream dependency**: This change should be applied after `org-infra` and `complytime-policies` changes land, so that testdata aligns with the production state. However, since the ID matching is string-based, the changes are mechanically independent.
