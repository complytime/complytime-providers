# Tasks

Implementation starts only after these spec artifacts are committed and pushed.
`[P]` marks tasks eligible to run in parallel. Do not start this work on a branch
that carries an unrelated feature change; the containment commits belong on their
own branch so review scope stays clean.

Tasks 1-7 record the earlier native-fork implementation and its reported
verification; none establishes plugin-first behavior. Task group 8 tracks the
2026-11-17 plugin-first revision and remains unimplemented. Commit and push the
revised spec before starting group 8 implementation. Do not change an existing
verification guard without maintainer approval.

## 1. Establish the durable fork

- [x] 1.1 Copy the working native command content for `uf.review-council.md`,
  `uf.review-pr.md`, and `uf.address-feedback.md` from the contributor's working
  tree onto this branch, add the `<!-- repo-owned fork -->` marker, and remove
  the UF scaffold marker. Historical native-only static check (superseded by
  the approval-gated path-isolation work in 8.2): the three files match none of
  `get_progress|checkpoint|clear_progress|plan_review_dispatch|dispatch_review_runs|consolidate_dispatch|finalize_review_dispatch|capture_review_diff|fetch_pr_review_context|submit_review_findings|submit_lesson_proposal|normalize_model_name|format_unleash_exit`.
- [x] 1.2 [P] Record the pinned version and per-file digests in
  `.uf/fork-manifest.sha256`, add `make verify-uf-fork` (GNU `sha256sum` or
  `shasum` fallback), and add `make update-uf-fork-manifest` so the fix path is
  one command. Verification: 7.2.
- [x] 1.3 [P] Document the pinned version, the fork marker, both Makefile
  targets, and the prohibition on `uf init --force` in `AGENTS.md` without
  changing any quality or governance gate.
- [x] 1.4 Add a repository-owned shell test for both targets under
  `.github/scripts/test/` covering: manifest pass, one-byte drift, missing
  manifest, dry-run prune, boundary retention, and refusal on tracked paths.
  Wiring it into a workflow is a CI change that needs maintainer approval and is
  not part of this change.

## 2. Report the execution path

- [x] 2.1 Add the capability preflight and the single execution-path banner to
  the Shared Execution Contract, and reference it from the other two commands.
  The banner distinguishes prompt protocol from runtime enforcement and requires
  the actual runtime version. Verification: 7.2.
- [x] 2.2 Remove unreported fallback paths: no parent-only review, verdict, or
  triage summary when neither the plugin nor the native path can complete the
  command. Verification: 7.2.

## 3. Keep tooling out of review scope

- [x] 3.1 Partition product, governance, and excludable-tooling paths in the
  Shared Execution Contract, keep governance-defining paths always in scope,
  report dependency paths separately, and report the counts before dispatch. Verification: 7.2.
- [x] 3.2 Require one explicit inclusion question for excludable tooling, with
  exclusion as the default and excluded paths recorded as unreviewed scope.
  Verification: 7.2.

## 4. Stop duplicate assessment on resume

- [x] 4.1 Persist per-item assessments keyed by `scope_digest` and a canonically
  serialized `item_digest`, and consume them on resume. Verification: 7.2.
- [x] 4.2 Re-assess only changed items, mark superseded results, and still
  require fresh consent at every gated action. Verification: 7.2.

## 5. Bound artifacts and provenance

- [x] 5.1 Stop writing caller transcripts, message dumps, and completion prompts
  into runtime records; require a measured per-invocation size in the report and
  an oversized-scope path that does not narrow reviewer scope. Verification: 7.2.
- [x] 5.2 Add `make clean-uf-workflows`: numeric argument validation, per-run
  depth, dry-run mode, failure propagation, tracked-path refusal, and retention
  of feedback posting receipts. Verification: 7.2.

## 6. Make the documentation true

- [x] 6.1 Rewrite `docs/review-command-compatibility.md`: correct the status,
  remove prototype-era leftovers, qualify non-repository paths, and keep
  prototype evidence separate from command compatibility claims.
- [x] 6.2 [P] Add the `CHANGELOG.md` entries and assess `README.md` impact.
  Verify `docs/AI_TOOLING.md` is not edited locally.
- [x] 6.3 Reconcile upstream drafts with the maintainer's disposition.
  Supersedes the earlier automatic-filing task: the maintainer managed the
  issues and explicitly requires preview and approval before any future
  issue/comment creation, update, or closure. The UF #962 comment was declined
  and not posted. References for the maintainer-managed issues:
  UF #1176, org-infra #747, and org-infra #746 (linked in the proposal).
  No further external posting is required for this PR.

## 7. Verify

- [x] 7.1 Run `openspec validate restore-plugin-free-review-commands --strict`,
  `make verify-uf-fork`, `git diff --check`, and the local CI commands derived
  from `.github/workflows/`. Report any unavailable tool or cross-repo blocker
  explicitly instead of marking it passed.
- [x] 7.2 Run the pre-PR council (review-council) on this branch and record the
  banner, governance inclusion, and consolidated verdict. Banner printed on the
  native path with no plugin tools; governance and tooling were included by
  explicit election because the change subject is tooling. Three review
  iterations and two resolution checks: iteration 1 (six personas) raised ten
  root causes; iteration 2 (adversary, testing) raised eight; iteration 3 found
  adversary, testing, and SRE approving, with guard, curator, and architect
  requesting documentation and scope-classification fixes. All findings were
  resolved, and the final resolution checks returned APPROVE from guard and
  architect, alongside the earlier APPROVE verdicts. Remaining divergence: the
  curator's three documentation items were fixed but not re-checked. Cover the banner (with a real version, no
  `<version>` placeholder), scope partition counts, governance inclusion, the
  resume-reuse path, the oversized-scope path, and the artifact ceiling. Also run
  the `verify-uf-fork` and `clean-uf-workflows` cases from 1.4 (the guard test
  covers them; 7.5 records the run). Record the runtime
  version and the observed output.
- [x] 7.3 Run a PR review smoke on a merged, small PR (no posting) and record the
  banner, scope partition, and dispatch outcome. Observations:
  `docs/review-command-compatibility.md` (Command smoke observations).
- [x] 7.4 Run a feedback triage smoke on a merged PR with comments, stopping
  before Phase 4, and record the banner, digests, and item classification.
  Observations: `docs/review-command-compatibility.md` (Command smoke
  observations).
- [x] 7.5 Run `bash .github/scripts/test/verify_uf_fork_test.sh` and record the
  pass.
- [x] 7.6 Resolve required findings from 7.2-7.5 and verify the approved scope is
  unchanged through submission. All REQUEST CHANGES findings were resolved before
  submission; the only post-approval commits are the documentation-only changes
  recorded in 7.2's resolution checks.

## 8. Prefer working plugins and retain a native fallback

- [x] 8.1 Inventory required tool contracts per command from the pinned UF
  v0.21.0 templates, including conditional phases, child submission, checkpoints,
  and feedback's post-fix council. Separate optional evidence from required tools.
  Specify a side-effect-free capability probe and compatible-contract criteria.
  Inventory: `docs/review-plugin-contracts.md`, inspected against tag target
  `ebed75730066865d777288a999aa3906f6a8b3f7`. Records template/schema
  inconsistencies without changing plugin sources, commands, or guards.
- [x] 8.2 Obtain and record maintainer approval before changing the existing
  fork guard's global plugin-tool rejection. If approval is absent, stop this
  subtask without disabling the guard. With approval, retain digest,
  missing-manifest, scaffold-drift, and cleanup coverage; replace the native-only
  assertion with path-aware checks and positive/negative fixtures for misplaced
  plugin calls or native dependence on plugin tools. Do not change CI or policy.
  Maintainer approval recorded on 2026-11-17: after being asked explicitly to
  approve this path-aware guard replacement, the contributor replied, "Yes, I
  am maintainer." Approval covers implementation maintenance preserving the
  existing review requirements, not changes to CI or governance policy.
  Implemented `.github/scripts/verify-uf-paths.awk`; the Make target still
  verifies digests and now checks explicit plugin/native sections. Positive,
  leakage, malformed-boundary, scaffold-restoration, and retained cleanup
  fixtures pass, as do `make verify-uf-fork` and shellcheck.
- [x] 8.3 Implement shared preflight for all three commands: prefer a complete
  compatible plugin contract regardless of runtime version; report capability
  gaps before selecting bounded native execution; stop when neither path works.
  Emit one truthful banner and bind the path before backend work.
  All three templates consume the shared inspection protocol and read-only
  `uf-review-preflight.jq` decision helper. Fixture selection is covered in 8.6;
  actual command acceptance remains 8.8.
- [x] 8.4 Restore complete plugin-backed workflows from pinned templates,
  including capture/ingestion, planning, dispatch, child submission, progress,
  consolidation/finalization, and post-fix council as applicable. Isolate native
  instructions and plugin-only skills. Keep scope, policy, verdicts, human gates,
  and artifact requirements common without changing their authority.
  Isolated plugin/native template branches restored, including capture/ingestion,
  progress, child submissions, finalization and cumulative post-fix council.
  Maintainer elected to keep plugin sources unchanged and report the pinned
  feedback behavioral gap as a preflight incompatibility, selecting bounded
  native execution when supported. See
  `docs/review-plugin-contracts.md#feedback-restoration-blocker`. No actual V1
  plugin-feedback compatibility is claimed; 8.8 remains open.
- [x] 8.5 Bind checkpoints, assessment provenance, and external receipts to the
  selected path. Preserve same-path digest reuse and fresh consent. Reject
  incompatible or unbound legacy state; stop when the saved backend is
  unavailable. Reconcile active children and ambiguous receipts before a fresh
  invocation; never automatically switch on plugin failures or adverse results.
  Versioned path bindings, backend progress references, cache provenance and
  receipt reconciliation are specified in the templates and checked through the
  shared read-only recovery decision helper; live artifact/receipt inspection
  remains the command's responsibility.
- [x] 8.6 Add deterministic fixtures for complete, partial, missing, incompatible,
  and ambiguous plugin capabilities; optional-tool absence; both/neither path
  usable; and plugin selection independent of reported V1/V2 version. Verify
  preflight creates no reviewers or backend checkpoints, and native execution
  invokes no plugin workflow tools.
  `uf_review_preflight_test.sh`: 79 passing decision fixtures, plus the path
  guard's native/shared leakage fixtures. Probe decisions create no files or
  backend actions. Static isolation and controlled decisions are not actual
  runtime execution evidence.
- [x] 8.7 Verify failures and recovery with controlled fixtures: plugin runtime
  error, policy rejection, unavailable model, blocking verdict, selected-backend
  loss, matching-digest resume, incompatible/legacy records, and post-fix council.
  Assert no backend switch, duplicate assessment, unauthorized mutation, or
  misleading plugin provenance. Capture external actions without real PR writes.
  `uf_review_recovery_test.sh`: 46 passing controlled decision fixtures, including
  failure events, path ownership, digest reuse, reconciliation and post-fix
  council. Outputs contain no backend actions and never authorize mutation.
  This does not claim a real plugin error, remote write, or reviewer lifecycle
  was exercised; those end-to-end observations remain in 8.8.
- [ ] 8.8 Exercise all three commands on an actual V1 runtime with the required
  plugins loaded and on V2 with plugins unavailable and bounded native execution.
  Record exact versions, tools used, structured outcomes, and failure behavior;
  mocked tests or lifecycle probes cannot establish command compatibility.
  Report missing runtimes/delegation as blocked or unverified, not passed.
  V2.3.0 live project-scoped capability inspection and a five-second bounded
  native lifecycle/cancellation fixture passed; exact caller model/variant and
  terminal state were verified. No configured V1 executable was identified.
  The native V2 council has additionally completed two real six-persona review
  iterations with verified identity, permissions and terminal state. Neither
  that council nor lifecycle evidence is full three-command compatibility;
  V1 and revised PR/feedback command acceptance remain unverified, so task stays
  open.
- [x] 8.9 Update `docs/review-command-compatibility.md`, `AGENTS.md`, and
  `CHANGELOG.md` to describe actual dual-path behavior and evidence; assess
  `README.md` impact and reconcile upstream drafts before filing. Preserve
  plugin sources/registrations and other UF commands. Regenerate the fork
  manifest in the same commit as intentional command edits.
  Updated documentation distinguishes controlled decisions, historical smoke
  evidence and pending runtime acceptance. README impact: none (provider usage
  unchanged). Upstream drafts now qualify historical measurements and the pinned
  feedback gap. Plugin sources/registrations and other UF commands are untouched.
  Intentional command edits and their regenerated manifest remain together in
  the working tree for one eventual implementation commit.
- [x] 8.10 Run strict OpenSpec validation, `git diff --check`, the approved fork
  guard and shell tests, and applicable local CI-derived checks. Run a fresh
  pre-PR council on the implementation and resolve all REQUEST CHANGES. Record
  unavailable tools and cross-repo blockers; earlier native-fork approvals do
  not approve this revision.
  Final local checks passed: 79 selection and 46 recovery fixtures, retained
  fork/cleanup guard, strict OpenSpec validation, whitespace check, build, unit
  tests, lint (zero issues), shellcheck, and four released-baseline CI helper
  suites. Fresh full-panel iteration 1 completed all six reviewers with verified
  native terminal state and returned REQUEST CHANGES on five root causes.
  Fixes cover confined literal-path cleanup, raw retention input, fail-closed
  Git inventory, oversized local-capture fidelity, and independent stale-digest
  fixtures. Full-panel iteration 2 completed all six reviewers and returned
  APPROVE WITH ADVISORIES, with no blocking findings. Its LOW-severity Design D7
  wording mismatch was aligned in a documentation-only follow-up; no
  implementation code changed after approval. Actual V1/V2 command acceptance
  remains explicitly unverified in 8.8; no full cross-repository host integration
  is claimed from the passing helper suites.
