## Context

The ampel provider's `convert` package loads granular policy JSON files, matches them against assessment configurations by ID, and merges matched policies into a bundle for the `ampel verify` tool. The test fixtures use `BP-X.YY` IDs that are being replaced with semantic slugs across the ecosystem.

The `LoadGranularPolicies` function currently reads a single flat directory (`os.ReadDir`), skipping subdirectories. This limits how policy source directories can be organized -- structured layouts (e.g., `branch-protection/`, `signed-commits/`) require a flat staging step before the provider can read them. This becomes relevant as `org-infra` moves to a grouped directory structure and future OCI distribution will extract content into structured directories.

## Goals / Non-Goals

**Goals:**

- Update all testdata to use new semantic IDs, aligned with the upstream org-infra and complytime-policies changes.
- Fix the pre-existing inconsistency where `ampel-bundle-expected-subset.json` contains older CEL expressions and single predicate URLs that don't match the actual source policies.
- Make `LoadGranularPolicies` support recursive directory walking so structured policy directories work without flat staging.
- Ensure `make test` passes with all changes.

**Non-Goals:**

- Changing `MatchPolicies`, `MergeToBundle`, or `WritePolicy` logic (matching is string-based, format-agnostic).
- Changing any Go code outside `convert/convert.go` and `convert_test.go`.
- Changing CEL expressions, predicate URLs, or assessment/error messages in the source policy files. Expected bundle fixtures are regenerated to match source policies, which fixes a pre-existing inconsistency in the subset fixture.
- Implementing OCI-based policy fetching (future feature, separate work).

## Decisions

### 1. ID Mapping

Following the Gemara semantic model established in the `test-branch-protection` catalog:

| Old Policy ID | New Policy ID | Old Control Ref | New Control Ref |
|---------------|---------------|-----------------|-----------------|
| `BP-1.01` | `require-pull-request` | `BP-1` | `pull-request-enforcement` |
| `BP-2.01` | `minimum-approvals` | `BP-2` | `approval-requirements` |
| `BP-3.01` | `block-force-push` | `BP-3` | `force-push-restriction` |
| `BP-4.01` | `prevent-admin-bypass` | `BP-4` | `admin-bypass-prevention` |
| `BP-5.01` | `require-code-owner-review` | `BP-5` | `code-owner-enforcement` |

OSPS framework references (`"framework": "OSPS", "class": "OSPS-QA", "id": "07"`) remain unchanged.

### 2. Recursive LoadGranularPolicies

**Decision**: Replace `os.ReadDir` + flat loop with `filepath.WalkDir` in `LoadGranularPolicies`.

The function will:
- Walk all subdirectories under the given root using `filepath.WalkDir` (not `filepath.Walk`). `WalkDir` does not follow directory symlinks, which is the desired safety behavior. The callback skips directory entries for policy processing (they are not JSON files) but does NOT return `filepath.SkipDir` -- it allows the walk to descend into them.
- Skip symbolic links to both directories and files (checked via `DirEntry.Type()` for `fs.ModeSymlink`).
- Apply the same filters: skip non-`.json`, skip `PolicyFileName`.
- Detect duplicate policy IDs: if two files at different paths share the same `id`, return an error naming both paths and the duplicate ID.
- Build the same `map[string]*AmpelPolicy` keyed by policy `id` field.
- Error handling: any read or parse error halts the walk and returns the error.

**Rationale**: Small, contained change. Forward-compatible with structured OCI extraction directories and org-infra's new `branch-protection/` grouping. Existing behavior is preserved when used with a flat directory (no subdirectories to recurse into). Symlink skipping prevents unintended file system traversal since the directory path can be user-controlled via the `ampel_policy_dir` global variable.

### 3. Expected Subset Bundle Fix

**Decision**: Regenerate `ampel-bundle-expected-subset.json` to match the actual output of `LoadGranularPolicies` + `MatchPolicies` + `MergeToBundle` using the current source policies (with new IDs).

The current subset expected file has stale content:
- Old ternary CEL: `has(...) ? ... : false` vs current compound: `(GitHub) || (GitLab)`
- Old single predicate URL: `.../specs/branch-rules.yaml` vs current dual: `.../specs/github/branch-rules.yaml` + `.../specs/gitlab/...`
- Slightly different description text.

This inconsistency predates our change. We fix it as part of this work since we're already touching all fixture files.

## Risks / Trade-offs

- **Test-only change**: No production Go code changes except `LoadGranularPolicies`. Risk of regression is low. Test code (`convert_test.go`) requires updating hardcoded `BP-*` ID string literals to the new semantic IDs -- no logic changes, only string updates.
- **Subset expected bundle regeneration**: The fix to `ampel-bundle-expected-subset.json` is a correctness improvement but changes the test baseline. If the test was accidentally passing with the wrong expected output, this could reveal a latent issue. Mitigation: run tests, inspect diff carefully.
- **Ordering**: The recommended deployment order is: org-infra first (canonical source), then complytime-policies (Gemara artifacts), then complytime-providers (testdata). The changes are mechanically independent (string-based matching), so each repo's change can be merged independently. However, testdata should be updated in the same timeframe to avoid divergence from the production state.
- **Rollback**: Each repository's change is self-contained and can be reverted independently. The string-based matching in `MatchPolicies` means any combination of old/new IDs works as long as the assessment plan IDs and granular policy IDs match. A partial rollback would leave IDs inconsistent across repos but would not cause runtime failures within any single repo.
