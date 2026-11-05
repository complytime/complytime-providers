# Tasks

Sections 1–3 record the initial implementation and its verification. Section 4
tracks the PR #233 review fixes and supersedes the initial exact-version policy
and version-related implementation tasks. Begin those fixes only after the revised
artifacts have been reviewed, committed, and pushed; do not change provider code in
the spec phase. Earlier successful runs remain historical evidence, not verification
of the pending revision.

## 1. Local input boundary

- [x] 1.1 Resolve the workspace boundary from the host-injected `workspace` base joined with complyctl's workspace directory name, with a configured-directory fallback, and treat the provider artifact directory as an additional boundary when it differs; add component-aware relationship handling and verify an enclosing target under the injected base, relative and absolute paths, equality, descendants, unrelated directories, custom workspaces, a missing variable, regex metacharacters, and similar-name siblings with unit tests.
- [x] 1.2 Reject explicitly selected workspace files and directories with actionable target-level operational errors; verify no assessments or synthetic passes are emitted and ordinary external files remain evaluable.
- [x] 1.3 Handle workspace and input symlink aliases, including symlinks to individual workspace files, directory aliases, symlinked workspace roots, loops, and unresolved paths; verify aliases are excluded or safely rejected without contaminating assessments.
- [x] 1.4 Detect hard-linked workspace files using filesystem identity for explicit local files, directory inputs, and files reached through symlinks in local or remote targets, omitting paths under a deliberately selected clone while building the comparison sets rather than subtracting identities; test safe exclusion or rejection, separate input directories, a clone file hard-linked to another workspace file, identity/traversal errors, an ordinary clone that must remain evaluable, and an unrelated external file.
- [x] 1.5 Implement cancellation-aware preflight without retaining file contents or repeated pairwise identity comparisons; test cancellation and benchmark a representative tree of at least 10,000 workspace files and 10,000 target files, recording each size, elapsed time, and memory use without adding a timing threshold to CI. Compare results against the design's review criterion and investigate overruns before merging.

## 2. Conftest input isolation

- [x] 2.1 Thread local-target exclusion through the runner without excluding intentional remote clones; test actual arguments for enclosing local directories, non-overlapping paths, single files, custom workspace paths, and remote targets. Add a hermetic target-level remote repository regression that verifies the clone is evaluated and produces the expected assessment.
- [x] 2.2 Derive and escape conftest ignore patterns for the walker's actual path forms; compose a nonempty `CONFTEST_IGNORE` with the workspace expression without mutating the environment, apply the workspace expression alone only when that variable is empty or unset and no configuration file applies, and otherwise reject the overlapping target with an actionable error naming the file, or the searched location and failure reason when detection failed, plus the remedies. Detect configuration using the conftest process working directory and environment, resolving relative `CONFTEST_CONFIG_FILE` paths and considering only filenames the verified version loads. Test nonempty, empty, and unset `CONFTEST_IGNORE` with and without a configuration file, explicit relative and absolute config paths, an unsupported `conftest.*` extension, a deterministic detection failure that must reject without assessments when no environment rule is set and compose when one is, non-overlapping targets, boundary behavior, spaces, and regex metacharacters.
- [x] 2.3 Add operational diagnostics and failure handling for unsafe alias resolution and filesystem traversal; test that broken links or unreadable paths do not become passing assessments and that errors contain actionable context. Inject traversal errors deterministically in unit tests; verify permission-fixture preconditions and skip such a fixture if the environment cannot enforce them.
- [x] 2.4 Update OPA man-page source and generated page and relevant README guidance to explain exclusion, which workspace directories are resolved and excluded when the host workspace and provider artifact directory differ, changed enclosing-directory results, explicit workspace target errors, empty local and remote inputs, hard links, remote-clone symlink checks, the copy-out workaround, non-pruned traversal, the conftest configuration rejection and its remedies, the verified conftest version requirement and remedy, and the limitation that preflight is not a sandbox against concurrent filesystem changes; verify examples against tests.
- [x] 2.5 Treat local and remote directory targets without confirmed evaluated eligible input as operational errors before synthetic-pass generation; test empty, workspace-only, fully ignored, empty remote clone or scan path, and mixed multi-target cases alongside an evaluated-file control.
- [x] 2.6 Detect conftest versions before local or remote OPA evaluation; reject unverified or undetermined versions through target-level operational errors without assessments. When the reported version is the source-build placeholder, fall back to the executable's embedded module version via `debug/buildinfo` and accept only a verified match. Test v0.68.2, other versions, failed or malformed version output, a source-built verified binary, a source-built unverified binary, an unreadable embedded version, and error details. Verify with a normal `complyctl scan` that the error is visible without debug logging and causes a nonzero exit, using a locally built complyctl, isolated temporary workspace and policy fixtures, and a deterministic fake conftest on PATH without network access during the scan.
- [x] 2.7 Resolve remote clone roots and scan paths and inspect traversed symlinks with visited-directory tracking; omit paths under the selected clone while building the workspace comparison sets, then reject workspace escapes, identity matches reachable outside the clone, symlink cycles, and unresolved paths before evaluation. Test file and directory symlinks into other workspace content, a selected scan path alias, a symlink to an externally hard-linked workspace file, a clone file hard-linked to another workspace file, a symlink cycle, non-cyclic internal clone symlinks, and an ordinary clone that still produces its normal assessments.
- [x] 2.8 Route every operational error added by this change through the Scan response rather than provider logs alone; verify with a normal `complyctl scan` that workspace, empty-input, alias, and compatibility errors are visible without debug logging, identify the affected target and remedy, exit nonzero, and preserve a successful target's assessments in a mixed scan.
- [x] 2.9 Add an end-to-end `complyctl scan` regression with a workspace base distinct from the working directory; verify both resolved workspace boundaries are excluded, eligible input outside them is still evaluated, and assessments reflect only that input.

## 3. Real-tool regression and release readiness

- [x] 3.1 Add a real-conftest regression with a valid triggering workspace fixture, a workspace fixture that would pass the policy, malformed workspace JSON, a valid external target, and repeated scans; verify evaluated filenames and success counts exclude workspace files, only intended inputs contribute findings, and the malformed workspace file is not parsed. Add an external file and policy that evaluates cleanly without findings; assert observed structured output and its passing assessment, contrasting empty and fully ignored inputs.
- [x] 3.2 Exercise real-conftest alias cases, including symlinks to workspace files and directories, a symlinked workspace root, hard-linked files, remote clone symlinks into other workspace content, and internal clone symlinks; verify no unintended workspace findings appear or the target fails safely, and verify ordinary external and remote targets still produce their expected results.
- [x] 3.3 Run the real-tool regression with conftest v0.68.2 and record the command, version, and result; if the optional test skips when conftest is absent, do not count the skip as validation. Include empty, workspace-only, and fully ignored directories and observe actual structured output. Exercise relative and absolute input paths, workspace regex metacharacters, and an existing effective ignore rule; assert intended eligible results and that workspace and user-ignored files contribute neither findings nor success counts. Document the coverage gap until resolved; do not change repository CI gates.
- [x] 3.7 Track a separate issue with `complyctl` harness maintainers for recurring non-skipping real-conftest coverage, and link it before implementation merge. Filed as [complyctl#957](https://github.com/complytime/complyctl/issues/957).
- [x] 3.8 File the required website issue for the migration guidance and link it before implementation merge. Filed as [website#189](https://github.com/complytime/website/issues/189), matching the `complytime/website` precedent in `CHANGELOG.md`.
- [x] 3.4 Run `openspec validate opa-exclude-workspace-from-input --strict`, `make build`, `make test`, `go test -race -count=1 ./...`, `make lint`, `make man`, and `git diff --check`; inspect applicable CI workflows and record any external prerequisites or unrun checks.
- [x] 3.5 Assess CHANGELOG, AGENTS, README, and website impact; document explicit-workspace-target errors, changed enclosing-directory results including user-owned files, the copy-out workaround, errors for empty or fully ignored local directories and for remote clones or scan paths with no evaluated input, the resolved workspace boundaries when the host workspace and provider artifact directory differ, remote-clone symlink checks, the conftest configuration rejection with its remedies, and the verified conftest requirement with guidance to install a verified version, including downgrading if necessary.
- [x] 3.9 Ensure the migration guidance reaches the generated GitHub release notes rather than relying on `CHANGELOG.md` alone, and record how it was verified. Verified against `.goreleaser.yaml`: notes come from commit subjects, so the implementation uses `fix(opa)!:` subjects, one per user-visible break, each matching the Bug Fixes group rather than being filtered. `.goreleaser.yaml` has no breaking-change group and no footer, so the subjects must carry their own remedies; adding the group and a guidance footer is tracked as [#232](https://github.com/complytime/complytime-providers/issues/232).
- [x] 3.6 Run the required review council before the implementation PR and resolve REQUEST CHANGES; confirm the evidence-retention change lands after this fix or contains equivalent protection.

## 4. PR review fixes

- [x] 4.1 Revise and strictly validate the proposal, design, and capability deltas for configured-target rejection across branches and behavior-based conftest compatibility; assess documentation impact without changing implementation or governance gates.
- [x] 4.2 Review, commit, and push the spec-only revision before beginning implementation. If push is not authorized, retain the local spec commit and pause at this boundary.
- [x] 4.3 Add failing regressions for a valid remote branch followed by an unsafe symlink or hard link, an empty or fully ignored directory branch, and missing evaluated-input evidence; repeat with reversed branch order, a separate valid configured target, an all-valid multi-branch control, and cancellation.
- [x] 4.4 Fix configured-target aggregation so the new input-safety and evaluated-input rejections discard assessments from all selected branches, retain actionable target/branch errors and accumulated result-write errors, and preserve other targets' assessments and mapping references. Use structured error classification rather than error-message matching; preserve unrelated legacy error behavior and existing artifact/clone lifecycle.
- [x] 4.5 Remove exact-version runtime enforcement and the source-build module fallback, clean up unused version-only mocks and tests, and replace CLI version-rejection assertions with behavior-based acceptance and operational-error assertions. Verify that different, unknown, and placeholder version strings do not independently block otherwise valid local or remote scans.
- [x] 4.6 Make real-tool helpers execute any installed conftest without version-based skips; skip only when absent, fail for discovered executables that cannot perform the required operations, and record available version diagnostics without requiring them. Add behavior contracts for flag/environment/config ignore precedence, config discovery, explicit-file ignore bypass, symlink traversal and resolution, structured evaluation evidence, and empty-input errors.
- [x] 4.7 Update CHANGELOG, root and provider READMEs, AGENTS usage/structure guidance, RPM description, and man-page source/generated output to remove exact-version and downgrade requirements, explain configured-target rejection across branches, and document tested compatibility and remaining automation limits. Keep recurring changing-release coverage scoped to the existing complyctl#957 follow-up and migration guidance to website#189; do not change CI or governance gates.
- [x] 4.8 Run strict OpenSpec validation, make build, make test, go test -race -count=1 ./..., make lint, make man, and git diff --check. Run real-conftest contracts against v0.68.2 and at least one other available release, recording actual versions, commands, outcomes, and any skipped prerequisites. Run offline complyctl regressions with rebuilt binaries, then rerun the required review council and resolve blocking findings before completing the fix commits.

## Review-fix verification

- The new multi-branch regression failed before the aggregation fix for workspace
  symlinks, hard links, empty or fully ignored branches, and missing evaluated-input
  evidence in both branch orders. The fixed implementation passes those cases and
  retains a separate valid target's assessments, evidence, and mapping references.
- `make build`, `make test`, `make lint`, strict OpenSpec validation, `make man`,
  and `git diff --check` passed. An unrelated Ampel man-page wrapping change from
  generation was excluded from this OPA-only change.
- `go test -race -count=1 ./...` passed with conftest 0.71.1 on `PATH`, including
  the real-tool regressions. Offline `TestComplyctlScan_Offline` tests passed with
  rebuilt complyctl/provider binaries, conftest 0.71.1, and Linux `bwrap`.
- The cross-repository integration workflow's `make test-cross-repo` passed with
  a locally built complyctl, provider binaries, snappy, ampel, and conftest 0.68.2.
- The real-tool suite command below passed separately against release binaries
  reporting `Conftest: 0.68.2` and `Conftest: 0.71.1`, with each executable placed
  on `PATH` for its run:

  ```bash
  go test -race -count=1 ./cmd/opa-provider/server \
    -run '^(TestScan_(RealConftest|.*WithRealConftest|WorkspaceIgnoreWithConftest)|TestRequireRealConftest)'
  ```

- Conftest 0.71.1 honors ignore for explicit files, whereas 0.68.2 bypasses it.
  The contract accepts either evaluation of only the explicitly selected file or
  an empty-input operational error, not an unrelated input or a synthetic pass.
  This is a safe behavior difference, not a reason to reject a release by version.
- Recurring non-skipping coverage of changing releases remains tracked in
  [complyctl#957](https://github.com/complytime/complyctl/issues/957). No CI or
  governance gates were changed; these runs do not guarantee future compatibility.
- All nine discovered Divisor reviewers approved the code review: adversary,
  architect, guard, testing, sre, curator, scribe, herald, and envoy. The guard's
  non-blocking changelog traceability advisory was addressed with spec references.
  No source changes followed approval.
- Gaze reported 90.5% OPA server statement coverage and full line coverage for
  the new error/filter helpers, with no concrete new correctness risk. Its
  quality/classification commands could not analyze Go 1.26 using a Go 1.25-built
  executable; baseline comparison also had absolute/relative path mismatches and
  unrelated Ampel/OpenSCAP violations. Those limitations were disclosed to the
  council; no baseline or threshold was changed. Windows server tests additionally
  cross-compiled successfully; this is not a Windows-native test run.

## Cyclic-clone review follow-up

- Reopened task 4.4 on 2026-11-04 against `cdbe413` and completed its correction
  in this follow-up commit to enforce the existing configured-target
  isolation requirement after PR review reproduced a rejected clone's directory
  cycle blocking unrelated targets. No new capability or specification requirement
  was introduced.
- Workspace identity collection now bounds traversal with its existing complete
  and clone-excluding partial visit sets. A complete alias walk can still collect
  identities omitted by a partial walk. Selected-input traversal continues to
  reject cycles; permission, resolution, identity, and cancellation errors still
  propagate. Rejected clones remain on disk.
- The cycle-rejection wording at `design.md:67-68` applies to selected-input
  inspection. Comparison-set collection bounds resolvable directory revisits
  without rejecting them. This note clarifies the two traversals without changing
  the committed specification or adding a new requirement during review fixes.
- Unresolvable link loops and workspace permission or identity failures remain
  fail-closed: workspace inspection rejects every local and remote target using
  that boundary, including all branches of each remote target. An ancestor alias
  that exposes a selected clone's files at another workspace path still causes an
  identity rejection, as required by the existing outside-clone alias contract.
- Earlier revisions of the regressions failed before the fix for both branch
  orders, mixed valid local and remote targets, subsequent scans, and workspace
  identity collection.
  They now pass, including retention of assessments, evidence, mapping references,
  regular-file identities, and identity matches reachable through cyclic aliases
  outside the selected clone.
- `make build`, `make test`, `go test -race -count=1 ./...`, `make lint`, and
  `openspec validate opa-exclude-workspace-from-input --strict` passed, as did
  `git diff --check`. Man-page sources are unchanged, so regeneration is unnecessary.
- The real-tool suite recorded above was rerun for this follow-up and passed
  without skips against the existing release binaries reporting conftest 0.68.2
  and 0.71.1. Offline
  `TestComplyctlScan_Offline` regressions passed with the rebuilt OPA provider,
  an existing locally built complyctl, conftest 0.71.1, and Linux `bwrap`.
- Gaze v1.8.0 rebuilt with Go 1.26 reported 88.2% average function line coverage
  and 82.1% for `collectDirectory`, whose complexity decreased by one. The internal
  identity-map contract is not mechanically measurable by Gaze; the direct map
  assertions provide its regression coverage. The existing baseline is not
  comparable to this package. No baseline, threshold, or CI gate was changed.
- All nine council reviewers approved this follow-up after two review iterations.
  Scope clarifications and documentation corrections resolved the blocking findings;
  no source or test changes followed the initial source review or final approval.
- Documentation impact: added a CHANGELOG fix entry. README, AGENTS, and man-page
  changes are unnecessary because this correction restores their existing
  independent-target guarantee without changing configuration, commands, or
  structure. Migration guidance remains covered by website#189.
- Hosted security/release checks and the cross-repository integration harness
  were not rerun locally for this follow-up; their prior results above are
  historical, not verification of this incremental fix.

## Initial implementation workflow record

- Commit the untracked `openspec/specs/target-scoped-generation/spec.md` baseline
  with these artifacts so the modified delta applies in a clean checkout.
- GoReleaser builds release notes from commit subjects only, so `CHANGELOG.md`
  entries and commit bodies do not reach them, and `^docs:` and `^test:` subjects
  are filtered out. Each break therefore needs one `fix(opa)!:` subject that
  carries its own remedy, because no footer links the fuller guidance:

  A six-way commit split was attempted and rejected: the breaks share
  `server.go` and `workspace.go`, so splitting them would either misattribute
  changes or produce commits that do not compile. The implementation therefore
  lands as one `fix(opa)!:` commit whose subject names the workspace exclusion,
  with the remaining breaks reaching users through `CHANGELOG.md`, the provider
  README troubleshooting table and the man page.

  This means the generated release notes disclose one of the five CHANGELOG
  breaks. Closing [#232](https://github.com/complytime/complytime-providers/issues/232),
  which adds a breaking-change group and a footer linking the full guidance, is
  the fix; until then the release-note gap is known and documented here.

  Before merge, confirm the PR title is a `fix(opa)!:` subject so a squash merge
  still produces a breaking entry rather than an unmarked one.
- Real-tool run recorded: `PATH=<conftest-0.68.2-dir>:$PATH go test -race -count=1
  ./cmd/opa-provider/server -run '^TestScan_(RealConftest|.*WithRealConftest|WorkspaceIgnoreWithConftest)'`
  passed against `Conftest: 0.68.2`. The suite skips when conftest is absent or
  another version is on `PATH`, and local CI does not install it, so recurring
  automated coverage remains a gap until the cross-repository follow-up lands.
- Opt-in CLI regressions recorded: `go test -race -count=1 ./cmd/opa-provider/server
  -run '^TestComplyctlScan_Offline'` passed with a locally built complyctl, the OPA
  provider binary, conftest 0.68.2 and `bwrap` on Linux. These skip by default.
- `complyctl doctor` still reports the provider healthy when an unverified conftest
  version is installed, because `Describe` checks tool presence only. Making it
  unhealthy would make complyctl skip the provider and suppress the target-level
  error this change requires, so the fix needs a complyctl-side decision. Raise it
  with the complyctl maintainers separately.
- Archive this change after implementation, verification, and review.
- Link [issue #231](https://github.com/complytime/complytime-providers/issues/231)
  and coordinate its closure with the implementation PR.

<!-- spec-review: passed -->
<!-- code-review: passed -->
