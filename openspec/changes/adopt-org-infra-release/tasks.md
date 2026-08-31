## 1. Rewrite release.yml

- [x] 1.1 Replace inline `preflight` job with caller to `complytime/org-infra/.github/workflows/reusable_release_preflight.yml@0c784711926c9864f027ec565fd7c06a382d80f8` (v0.7.1), passing `tag: ${{ inputs.tag }}` and `ci_checks: '["Build and test", "Standardized CI / Run linters"]'`, with permissions `contents: write` and `checks: read`
- [x] 1.2 Add `skip_semver_check`, `skip_ci_checks`, and `skip_unreleased_check` as `workflow_dispatch` boolean inputs (default `false`) and forward them to the reusable preflight
- [x] 1.3 Replace inline `release` job with caller to `complytime/org-infra/.github/workflows/reusable_release_goreleaser.yml@0c784711926c9864f027ec565fd7c06a382d80f8` (v0.7.1), passing `tag: ${{ needs.preflight.outputs.tag }}`, with `needs: preflight`, `if: needs.preflight.outputs.tag != ''`, and permissions `contents: write` and `id-token: write`
- [x] 1.4 Set top-level permissions to explicit deny (`contents: none`, `id-token: none`), matching complyctl pattern
- [x] 1.5 Remove duplicate `concurrency` block from caller (reusable preflight defines its own)
- [x] 1.6 Add header comments with issue reference and description of what the workflow delegates to

## 2. Verification

- [x] 2.1 Verify `.goreleaser.yaml` compatibility with reusable GoReleaser workflow (cosign + syft sections already present)
- [x] 2.2 Validate `release.yml` YAML syntax (`python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` or `actionlint`)
- [x] 2.3 Verify workflow references match pinned SHA with version comment (`# v0.7.1`)
- [x] 2.4 Confirm no other files reference inline preflight logic that needs updating

## 3. Documentation

- [x] 3.1 Update `AGENTS.md` CI Workflow Structure table to reflect the rewritten `release.yml` (reusable workflow callers, not inline jobs)
