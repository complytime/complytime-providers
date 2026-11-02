# Tasks

## 1. Upstream Baseline Readiness

- [ ] 1.1 Verify complyctl PR #944 is merged and confirm its merged documentation still assigns downstream provider build and runtime validation to issue #187; record the merged commit and verify with `gh pr view 944 --repo complytime/complyctl`.
- [ ] 1.2 Inspect the current stable complyctl release for the cross-repository integration prerequisites and verify the chosen release can provide the required build target and test harness.

## 2. Released Compatibility Workflow

- [ ] 2.1 Add a fail-closed workflow step or job that derives the direct complyctl dependency major from `go.mod`, selects the highest stable matching tag, and reports its tag and commit; verify normal, prerelease, malformed-tag, and no-baseline behavior.
- [ ] 2.2 Add a released-baseline integration job that checks out the selected complyctl release, preserves the existing main-branch integration job, and runs provider build plus the cross-repository integration target; verify the workflow identifies the release checkout separately.
- [ ] 2.3 Add explicit released-checkout prerequisite validation and stage-specific failure diagnostics; verify missing prerequisites, provider build failures, and integration failures report distinct messages with the selected tag.
- [ ] 2.4 Validate the modified workflow with `yamllint .github/workflows/ci_cross_repo_integration.yml` and an available GitHub Actions workflow linter, then verify a CI run publishes the baseline tag and commit.

## 3. Verification and Documentation Assessment

- [ ] 3.1 Run `make build`, `make test`, and `make lint` after the workflow change; verify all existing provider checks remain green.
- [ ] 3.2 Assess README.md, AGENTS.md, CHANGELOG.md, and provider documentation for the released-compatibility workflow impact; update only documentation that changes contributor or repository behavior, and verify documented commands are accurate.

## Workflow follow-up

- Run the required review process after implementation.
- Archive the change after the project's review requirements are satisfied.
