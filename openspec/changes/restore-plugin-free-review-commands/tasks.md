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
- [ ] 6.3 File the four drafts in `issue-drafts.md` (UF #962 comment, UF
  scaffold-drift issue, `org-infra` evaluation issue) and record their URLs here
  and in the proposal.

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

- [ ] 8.1 Inventory required tool contracts per command from the pinned UF
  v0.21.0 templates, including conditional phases, child submission, checkpoints,
  and feedback's post-fix council. Separate optional evidence from required tools.
  Specify a side-effect-free capability probe and compatible-contract criteria.
- [ ] 8.2 Obtain and record maintainer approval before changing the existing
  fork guard's global plugin-tool rejection. If approval is absent, stop this
  subtask without disabling the guard. With approval, retain digest,
  missing-manifest, scaffold-drift, and cleanup coverage; replace the native-only
  assertion with path-aware checks and positive/negative fixtures for misplaced
  plugin calls or native dependence on plugin tools. Do not change CI or policy.
- [ ] 8.3 Implement shared preflight for all three commands: prefer a complete
  compatible plugin contract regardless of runtime version; report capability
  gaps before selecting bounded native execution; stop when neither path works.
  Emit one truthful banner and bind the path before backend work.
- [ ] 8.4 Restore complete plugin-backed workflows from pinned templates,
  including capture/ingestion, planning, dispatch, child submission, progress,
  consolidation/finalization, and post-fix council as applicable. Isolate native
  instructions and plugin-only skills. Keep scope, policy, verdicts, human gates,
  and artifact requirements common without changing their authority.
- [ ] 8.5 Bind checkpoints, assessment provenance, and external receipts to the
  selected path. Preserve same-path digest reuse and fresh consent. Reject
  incompatible or unbound legacy state; stop when the saved backend is
  unavailable. Reconcile active children and ambiguous receipts before a fresh
  invocation; never automatically switch on plugin failures or adverse results.
- [ ] 8.6 Add deterministic fixtures for complete, partial, missing, incompatible,
  and ambiguous plugin capabilities; optional-tool absence; both/neither path
  usable; and plugin selection independent of reported V1/V2 version. Verify
  preflight creates no reviewers or backend checkpoints, and native execution
  invokes no plugin workflow tools.
- [ ] 8.7 Verify failures and recovery with controlled fixtures: plugin runtime
  error, policy rejection, unavailable model, blocking verdict, selected-backend
  loss, matching-digest resume, incompatible/legacy records, and post-fix council.
  Assert no backend switch, duplicate assessment, unauthorized mutation, or
  misleading plugin provenance. Capture external actions without real PR writes.
- [ ] 8.8 Exercise all three commands on an actual V1 runtime with the required
  plugins loaded and on V2 with plugins unavailable and bounded native execution.
  Record exact versions, tools used, structured outcomes, and failure behavior;
  mocked tests or lifecycle probes cannot establish command compatibility.
  Report missing runtimes/delegation as blocked or unverified, not passed.
- [ ] 8.9 Update `docs/review-command-compatibility.md`, `AGENTS.md`, and
  `CHANGELOG.md` to describe actual dual-path behavior and evidence; assess
  `README.md` impact and reconcile upstream drafts before filing. Preserve
  plugin sources/registrations and other UF commands. Regenerate the fork
  manifest in the same commit as intentional command edits.
- [ ] 8.10 Run strict OpenSpec validation, `git diff --check`, the approved fork
  guard and shell tests, and applicable local CI-derived checks. Run a fresh
  pre-PR council on the implementation and resolve all REQUEST CHANGES. Record
  unavailable tools and cross-repo blockers; earlier native-fork approvals do
  not approve this revision.
