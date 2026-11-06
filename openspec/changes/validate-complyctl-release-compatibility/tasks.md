# Tasks

## 1. Upstream Baseline Readiness

- [x] 1.1 Verify complyctl PR #944 is merged and confirm its merged documentation still assigns downstream provider build and runtime validation to issue #187; record merge commit `b6aebd721deebf05d48a17c8e5fb25b3c2e271ee` and verify with `gh pr view 944 --repo complytime/complyctl`.
- [x] 1.2 Inspect the current stable complyctl release for the cross-repository integration prerequisites and verify the chosen release can provide the required build target and test harness.

## 2. Released Compatibility Workflow

- [x] 2.1 Add a fail-closed workflow step or job that derives the direct complyctl dependency compatibility line from `go.mod` (major for v1+, minor for v0), selects the highest stable matching tag, and reports its tag and commit; verify normal, prerelease, malformed-tag, v0 minor-boundary, and no-baseline behavior.
- [x] 2.2 Add a released-baseline integration job that checks out and verifies the selected complyctl commit, preserves the existing main-branch integration job, builds and tests all providers from an isolated workspace with a temporary replacement to that checkout, then runs the released checkout's Ampel-focused cross-repository integration target; verify the workflow identifies the release checkout separately and rejects a commit mismatch.
- [x] 2.3 Add explicit released-checkout prerequisite validation and stage-specific failure diagnostics; verify missing prerequisites, provider build failures, and integration failures report distinct messages with the selected tag.
- [ ] 2.4 Validate the modified workflow with `yamllint .github/workflows/ci_cross_repo_integration.yml` and an available GitHub Actions workflow linter, then verify a CI run publishes the baseline tag and commit.

## 3. Verification and Documentation Assessment

- [x] 3.1 Run `make build`, `make test`, and `make lint` after the workflow change; verify all existing provider checks remain green.
- [x] 3.2 Assess README.md, AGENTS.md, CHANGELOG.md, and provider documentation for the released-compatibility workflow impact; update only documentation that changes contributor or repository behavior, and verify documented commands are accurate.

## Workflow follow-up

- Run the required review process after implementation.
- Archive the change after the project's review requirements are satisfied.
