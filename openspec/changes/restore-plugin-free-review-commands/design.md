# Design

## Context

See [proposal.md](proposal.md) for motivation and
[the compatibility spec](specs/review-command-compatibility/spec.md) for the
behavior contract.

The runtime measured when the fork was created was OpenCode v2.2.7; the recorded
command smoke checks ran on v2.3.0. The scaffolded plugin dependency is
`@opencode-ai/plugin` 1.19.0, and all four UF plugins fail to load with
`PluginModule.LoadError`. UF v0.21.0 is the latest release; the V2 migration
(#714, #797) is open and unreleased, so a scaffold update cannot resolve this
now.

The previous revision of this change proposed a plugin-free prototype with a
cross-version deadline supervisor and V1 context discovery. That work produced
useful evidence but did not repair the commands: after six sessions the branch
held 3 of 15 completed tasks and no command changes. Meanwhile the contributor's
working adaptations accumulated uncommitted. This revision chooses containment.

The 2026-11-17 revision extends containment to plugin-first execution. The
implemented native fork names a plugin path in its banner but unconditionally
instructs native orchestration. This design restores a complete plugin-backed
path for compatible contributors and keeps native execution as a capability-based
fallback. It specifies future work; the existing native smoke checks do not
verify the revised contract.

### Evidence base

The proposal's measurements come from:

- `~/.local/share/opencode/log/opencode.log`: 247 plugin-load warnings, all four
  plugins, same `PluginModule.LoadError` cause.
- The local OpenCode session store for this project: 29 top-level and 52 child
  sessions created after 2026-11-05 16:27 UTC. Tool-name inventory shows no
  plugin tool call; child reviewer sessions were created by shell-driven API
  calls.
- Ignored runtime records under `.uf/workflows/` and `.uf/feedback/`: 60 MB
  across eight runs, up to 17 MB and 46 files per run.

These are local observations, not published transcripts. Re-verification uses
disposable fixtures; no raw session content or credentials are committed.

## Goals / Non-Goals

**Goals:**

- Keep the three commands usable and honest on the runtime contributors actually
  run, without waiting for an upstream release.
- Retain plugin-backed execution when its complete tool contract is available,
  including on OpenCode V1, without hard-coding selection to a runtime version.
- Make the local adaptation durable against scaffolding and re-runs.
- Remove the friction patterns measured above: unreported fallback, scope
  collisions, duplicate assessment after resume, unbounded artifacts.

**Non-Goals:**

- Port the four plugins, or implement UF's migration.
- Change review eligibility, model policy, limits, verdicts, severity, packs,
  personas, permissions, or the constitution.
- Support every OpenCode generation. Claims are limited to versions actually
  exercised.
- Repair `/uf.unleash`, `/uf.finale`, `/uf.triage-issue`, or Speckit dispatch.
- Evaluate or adopt another agent platform. That is an organization-level
  decision recorded in `issue-drafts.md`.

## Decisions

### D1: Keep the `/uf.*` command names and own the content locally

The three command files keep their names. Their content becomes a repository
fork of the UF v0.21.0 scaffold, marked with `<!-- repo-owned fork -->` and
pinned to v0.21.0.

Alternatives considered:

| Option | Benefit | Cost |
| --- | --- | --- |
| Keep `/uf.*`, own content (chosen) | No `AGENTS.md` gate rename, no org-infra doc change, no retraining | Collides with future `uf init`; mitigated by D2 |
| Rename to `/review-council` etc. | Clean separation from scaffold | Requires an `org-infra` docs PR and `AGENTS.md` gate rewrite |
| Thin wrapper delegating to a native file | Preserves names and separation | Extra indirection, still overwritten by name |

Upgrade trigger: when UF ships a V2-compatible release, compare its three
commands with the fork, reconcile deliberately, and delete the fork marker and
manifest in the same change.

### D2: Detect scaffold drift instead of forbidding updates

Record the expected digest of the three files in `.uf/fork-manifest.sha256`
(`sha256sum -c` format) and add `make verify-uf-fork`. The target fails when a
command changes without a manifest update, which is exactly what an unexpected
`uf init` would do.

This is a local verification target, not a CI gate, so no quality or governance
gate is modified. `AGENTS.md` documents that `uf init --force` must not run until
the upstream upgrade trigger is met.

The existing target also rejects backticked plugin-tool names. That native-only
assertion conflicts with the dual-path contract. Before changing this guard,
obtain maintainer approval for a path-aware replacement; absent approval, stop
the guard work rather than disabling verification. Preserve digest checks,
missing-file failures, and scaffold-overwrite detection. Add checks that plugin
calls belong to the plugin branch and that native execution has no required
plugin call, with negative fixtures for cross-path leakage. Keep this a local
check; do not change CI, review limits, severity, or other governance policy.

### D3: Prefer a complete plugin contract; otherwise select native execution

Keep shared scope, council authority, policy sources, verdicts, human gates,
assessment reuse, and artifact rules common to both paths. Put backend-specific
capture, ingestion, planning, dispatch, submission, progress, and finalization
instructions in explicit plugin/native branches. Restore plugin behavior from
the pinned v0.21.0 templates without undoing the shared containment safeguards.

Before input capture, feedback ingestion, checkpoint writes, or child creation:

1. Inspect the current session's registered tools and exposed contracts. Determine
   the complete required set for the command, including enabled conditional
   phases, injected child submission calls, recovery, and the post-fix council.
   Plugin configuration or a successful load log alone is not sufficient.
2. Prefer the plugin path when all required tools expose compatible contracts.
   Do not require native delegation or native caller-discovery support on this
   path. Optional evidence remains optional under existing policy.
3. If required tools are missing or their exposed contracts are incompatible,
   record the specific gaps and evaluate the native path. Partial plugin loading
   never permits combining plugin planning with native dispatch or checkpoints.
   Ambiguous capability discovery stops rather than assuming absence.
4. Select native execution only when its delegation, model/variant, permissions,
   deadline, cancellation, and observable-terminal-state controls are supported.
   Otherwise report `UNAVAILABLE` and create no reviewers or mutations.

Inspect capabilities without dispatching probe reviewers or writing plugin
progress. Runtime version is diagnostic metadata, not a selector: compatible V1
uses plugins, V2 without plugins uses native execution, and a future compatible
V2 plugin set also uses plugins.

Retain original plugin-backed capture/ingestion, planning, dispatch,
consolidation, finalization, checkpoints, and child structured submission in the
plugin branch. The native branch must not invoke those plugin tools or require
the plugin-backed dispatch skill. Preserve required policy checks on both paths;
an unsupported required policy is a blocker, not grounds to bypass it.

#### Native execution controls

Use the native delegation tool when it can express the run. Where a configured
deadline cannot be expressed natively (the V2 subagent tool exposes no per-run
deadline), use the bounded shell/API supervisor already proven in
`docs/review-command-compatibility.md`, and record the child session, requested
and resolved model/variant, deadline, and verified terminal state.

Rules that apply to both paths:

- Never treat a killed client, an HTTP success, or an agent's promise to stop as
  cancellation evidence.
- Unconfirmed cancellation is a capability failure: stop progression and name the
  child rather than reporting success.
- Failed children are non-voting and named; they never become approvals.

### D3a: Collect reproducible capability evidence before applying selection

The 2026-11-18 specification revision addresses
[PR review 5943477705][preflight-review]. The reviewer reports that registered
tools still leave child visibility and lifecycle contracts unknown, stopping
`/uf.review-pr 244` before review. The current selector correctly preserves the
uncertainty; changing `unknown` to absence or compatibility would hide the gap.
The missing component is repeatable evidence acquisition, not a less restrictive
selection rule. This section specifies pending implementation, not verified
runtime support.

Implement a repository-owned evidence collector/verifier with a documented entry
point and regression tests. It supplies the existing decision input; the pure jq
selector keeps its selection rules and does not acquire runtime side effects.
Prefer the runtime's supported discovery/API interfaces. Consult the exact live
schema rather than assuming an endpoint, response shape, or V1/V2 equivalence.
If those interfaces cannot establish a required property, preserve `unknown` and
name the unsupported observation.

#### Capability acceptance and authoritative sources

The contract inventory will map every required key to an exact observation,
pass/fail criteria, source reference, and verifier case. Use this minimum mapping:

| Capability | Evidence source and acceptance |
| --- | --- |
| Required parent tools and schemas | Complete current-session callable inventory and exposed schemas, checked against the pinned contract. Parent registration establishes neither child visibility nor successful execution. |
| `immutable_scope` | Identified loaded dispatch implementation plus controlled command receipts showing all children consume the same frozen diff, inventory, and SHA pair. |
| `child_contract` | Effective tool/permission evidence for the actual reviewer persona and controlled findings-submission and clean-review receipts harvested into the correct run. A clean review must not fabricate findings or submit a schema-invalid empty array. |
| Plugin `lifecycle` | Controlled dispatch receipts showing observed model/variant, inherited permissions, enforced deadline, cancellation, and independently observed terminal state for the exact runtime and loaded implementation. |
| Native controls | Project-scoped supported delegation/session interfaces and effective persona definitions, with controlled receipts for `model_variant`, `permissions`, `deadline`, `cancellation`, `terminal_state`, and trustworthy `caller_binding`. Validate `delegation` itself against the exposed interface. |
| `policy` | Current authoritative matrix, overrides, capability manifest, and persona definitions, plus evidence that the selected execution mechanism preserves their configured requirements. |
| `feedback_assessment` | Identified loaded trusted child-response/consolidation implementation and actual feedback-command receipts preserving item assessment fields. Retain the pinned implementation's documented incompatibility. |
| `frozen_sibling_evidence`, when configured | Identified loaded aggregate implementation and receipts establishing one invocation-wide frozen bundle, not reacquisition per item or iteration. |

Source inspection supplies implementation identity and static semantics, not
proof that a particular child can call a tool or that a deadline fired. Runtime
receipts supply observed behavior, not a waiver of incompatible static contracts.
Handwritten status assertions, model prose, version strings, and fixture-only
successes cannot supply missing runtime evidence.

#### Separate collection from controlled runtime acceptance

Production preflight remains read-only: discover supported interfaces, read
current policy and eligible evidence, validate bindings, and emit deterministic
capability JSON with source references and a named reason for every gap. It must
not create reviewers, call workflow tools, write backend progress, or mutate
GitHub. Malformed, incomplete, ambiguous, or inaccessible observations fail closed.
Do not automatically install packages or start a replacement runtime.

Obtain behavioral receipts through a separate, explicitly invoked acceptance
harness in disposable repositories with isolated configuration/state. The harness
tests the specified controls without issuing a production review verdict or
authorizing a production invocation. It is not a fallback for a failed preflight.
Keep unchanged persona permissions, model selection, policy, and plugin sources;
capture attempted external actions with controlled fixtures rather than real
pushes, comments, reviews, or thread resolutions.

Bind each receipt to its runtime/service identity, project context, loaded plugin
identity where applicable, exposed-contract digest, policy/agent digests,
execution mechanism, scenario, outcome, and concrete observation references.
Production preflight must verify that the evidence applies to its current
context; a copied receipt or the contributor's assertion is not verification.
Changes to a bound identity or contract invalidate affected evidence, and absent
or unverifiable loaded-source identity remains unknown. Never assume installed
source is the source loaded by the running service.

Store evidence privately with directories `0700` and files `0600`, excluding
credentials and transcripts. Document the exact collection and acceptance
commands, their inputs/outputs and side effects, and how a new session verifies
and refreshes evidence. Identical normalized observations must produce identical
capability decisions. Positive runtime claims require inspectable observation
receipts; filenames, digests, or a self-reported `passed` field alone do not
establish provenance.

#### Acceptance boundary

Exercise all three real command flows for each claimed supported runtime/backend,
including findings, clean results, timeout/cancellation, terminal-state recording,
and declined mutation gates. Include the reported registered-tools/unknown-child
case, stale evidence, missing observation interfaces, and known incompatible
contracts. Record commands and exact versions in the compatibility page. A
missing V1 runtime remains unverified; an isolated lifecycle test alone does not
complete task 8.8. No source changes in this specification revision are authorized
until these artifacts are committed and pushed.

[preflight-review]: https://github.com/complytime/complytime-providers/pull/244#pullrequestreview-5943477705

### D4: Announce the selected path and never switch after execution starts

Each command applies the Shared Execution Contract and prints exactly one banner.
For a usable plugin contract:

```text
Execution path: plugin
Protocol (prompt-enforced): frozen review scope, council participation, mutation gates
Runtime-enforced: plugin dispatch, plugin artifacts, plugin progress checkpoints
Not provided: none
```

Report only plugin controls actually supplied by the verified contract; the banner
does not turn command-level protocol into plugin enforcement. For a usable native
fallback:

```text
Execution path: native (UF plugins unavailable on OpenCode v<major>.<minor>.<patch>)
Protocol (prompt-enforced): frozen review scope, council participation, mutation gates
Runtime-enforced: none
Not provided: plugin dispatch artifacts, plugin progress checkpoints
```

Include the recorded plugin capability gaps alongside the native banner, also
when some plugin tools loaded. The banner separates what the template instructs
from what the runtime enforces; a protocol step is never described as enforced.
A runtime with neither complete path reports `unavailable` and stops, and an
unreadable runtime version is reported as `vunknown` rather than invented.

Bind the selection before the first backend operation and keep it for all
iterations, child prompts, recovery phases, and nested post-fix council work in
that invocation. A plugin error, provider/model failure, timeout, invalid result,
policy rejection, or adverse verdict follows the existing selected-path failure
and partial-success rules. None authorizes switching to native execution.
If a required capability disappears after selection, stop progression and retain
the state for inspection. A new invocation may select a different path only after
recorded children and ambiguous external-action receipts are reconciled; it must
not replay side effects or silently import the other backend's completion state.
This prevents the PR #230 pattern of declaring `INCONCLUSIVE` and then producing
a full review without the council.

### D5: Keep excludable tooling out of review scope without hiding the gates

Classification:

- **Governance scope, always included**: `.specify/memory/constitution.md`,
  `.opencode/agents/`, `.opencode/uf/packs/`, `.uf/review-matrix.yaml`,
  `.uf/reviewer-capabilities.yaml`, `AGENTS.md`, and the three review commands.
  A review that excludes the files defining its own gates cannot claim to check
  them.
- **Dependency scope**: `vendor/`, `go.sum`, and lockfiles. Reported separately;
  reviewers check manifest/version consistency and upstream provenance rather
  than vendored third-party code, and a dependency-only change does not trigger
  Tier 2 unless the manifest change is security-relevant.
- **Excludable tooling**: other paths under `.opencode/`, `.uf/`, `.specify/`,
  `openspec/`, `docs/`, and `.github/`.
- **Product scope**: everything else.

Behaviour:

1. Compute the candidate scope, then partition it into product, governance,
   dependency, and excludable tooling files.
2. Report the four counts before dispatch, always.
3. Exclude excludable tooling from the frozen review diff and state the exclusion
   as unreviewed scope.
4. If excludable tooling is the intended subject (for example this change), ask
   the contributor once through the question tool whether to include it. Never
   infer consent.

This removes the measured failure mode where an uncommitted command edit and a
convention-pack change entered an 80-file review and produced a governance detour
with no source progress, without creating a path that reviews a command change
while ignoring the command.

### D6: Key assessments by scope digest so resume cannot double-charge

Persist one assessment record per grouped feedback item, containing the frozen
scope digest, the item text digest, execution path, policy/agent-definition
digests, reviewer runs, and the consolidated recommendation. Plugin execution uses
plugin checkpoints and validated artifacts; native execution uses native phase
records. A minimal shared binding records invocation, selected path, scope and
policy digests, backend record references, active-child references, and external
receipts without credentials or reusable consent. On resume:

- Matching execution path, scope/item digests, policy/agent definitions, and
  validated provenance: reuse the assessment and say so.
- Changed item or scope: re-assess only the affected items.
- Missing or corrupt assessment with an otherwise validated binding: reconcile
  active runs and receipts, then re-assess and mark the assessment reconstructed.
- Missing path provenance or an incompatible backend record: stop recovery for
  reconciliation. Never relabel native JSON as plugin-validated evidence or
  interpret a plugin checkpoint as a completed native phase.
- Selected-path capability unavailable on resume: stop; do not reselect a backend
  or repeat completed dispatch. Reconcile active children and receipts before any
  fresh invocation, retaining fresh human confirmation at every mutation gate.

This targets the measured duplicate dispatch, where two feedback items were
assessed twice at roughly double the parent cost.

### D7: Bound runtime artifacts

Retain the path binding, selected backend's required progress/validation artifacts,
the frozen scope reference, and assessment records. Do not
copy caller message dumps, completion prompts, or full transcripts into the
workflow directory. Measure the invocation directory after each phase and report
the measured size; 5 MB is a budget, not a runtime control, so the report must
state the measured value rather than assert compliance.

When the frozen scope exceeds the budget, do not copy it twice and do not narrow
what reviewers see. Give them the exact base/head SHAs, complete file list and
capture digest. For local captures containing working-tree or feature-directory
content, use bounded targeted reads of the existing frozen capture or immutable
included-file snapshots. Commit-only diffs cannot reconstruct those bytes.
Reading `git diff <base_sha>...<head_sha> -- <path>` is an alternative only for
fully committed captures whose equivalence to the frozen scope is verified.

`make clean-uf-workflows` lists per-invocation directories under
`.uf/workflows/` that have no write in the last
`UF_WORKFLOW_RETENTION_DAYS` days and deletes them only with `PRUNE=1`. It
validates and bounds the day argument, refuses when the tree contains tracked
files or Git cannot confirm that, and propagates failures. A record exactly at
the boundary is treated as expired. Feedback posting receipts under
`.uf/feedback/` are never pruned because they prevent duplicate replies.
Durable triage artifacts under `.uf/artifacts/` are retained by design and are not
pruned by that target; their growth is reported, not silently reclaimed.

`.uf/workflows/`, `.uf/feedback/`, and `.uf/artifacts/` remain ignored; no runtime
artifact is committed. Native evidence identifies itself as native and never
claims plugin validation.

### Safeguard mapping

Sources stay authoritative; this change reads them rather than copying policy
values. The mapping records which safeguard the contained path relies on.

| Safeguard and source | Contained-path mechanism |
| --- | --- |
| Eligibility: `.uf/reviewer-capabilities.yaml` | Read the manifest; content personas never vote; scope intersections preserved |
| Selection: default and full panel | Preserve configured selection; `--full` runs each eligible review persona once; never truncate |
| Models/variants: `.uf/review-matrix.yaml` and ignored override | Read authoritative configuration; report unsupported selection instead of substituting |
| Limits: matrix `limits` | Validate the full run list before dispatch; batch at or below the configured maximum; report skipped runs |
| Deadline: matrix `per_run_timeout_seconds` | Native path enforces it through the bounded supervisor (D3) and verifies terminal state |
| Reported-cost budget: matrix `max_reported_cost_usd` | Sum reported cost after each batch; unknown stays unknown |
| Immutable scope: council input resolution | Freeze base/head plus included working tree once; recheck before approval or mutation |
| Verdicts: command consolidation and `severity.md` | Structured returns only; deduplicate by root cause; partial-success and no-success rules retained |
| Recovery: selected backend | Path-bound plugin checkpoints or native phase records; digest-keyed reuse and fresh consent (D6) |
| Pre-flight/context: `pre-flight` and `review-context` skills | Unchanged; both already use native tools |
| Mutation gates | Question-tool confirmations, per-finding spec decisions, iteration bounds, post-fix council unchanged |
| Permissions/provenance: persona frontmatter | Unchanged personas and permissions; requested and observed identity recorded separately |
| Optional evidence: `.uf/sibling-repos.yaml`, Dewey, Gaze | Preserve configured policy; unavailable capability is reported, never replaced by another model |

## Risks / Trade-offs

- **A local fork of scaffolded files drifts from upstream.** Mitigation: pinned
  version, marker, digest manifest, and an explicit reconciliation procedure at
  upgrade time.
- **Fail-loud banners can be ignored by a model.** Mitigation: the banner is a
  required output line checked by the command's own verification step and by a
  fixture; static text alone is not treated as proof.
- **Scope exclusion could hide a genuine tooling review.** Mitigation: the
  contributor is explicitly asked when tooling files are the subject; exclusion
  is always reported.
- **Assessment reuse could serve stale conclusions.** Mitigation: reuse requires
  matching scope and item digests; anything else re-assesses.
- **The native path is less mechanically enforced than plugin dispatch.** Already
  true today; this change makes the gap visible and bounded instead of silent.
- **Dual paths can drift or duplicate work.** Mitigation: common policy and scope
  rules, per-command capability inventories, path-bound recovery, no automatic
  mid-invocation fallback, and separate integration evidence for each backend.

## Migration Plan

1. Commit and push the revised spec artifacts before changing command mechanics.
   Keep tasks 1-7 as historical native-fork evidence; track this revision in 8.
2. Obtain maintainer approval before replacing the guard's native-only assertion.
   Preserve its drift checks and add path-isolation regression fixtures.
3. Restore the plugin branch from pinned templates, isolate native instructions,
   and implement capability selection, path-bound recovery, and shared safeguards.
4. Update the manifest in the same implementation commit as intentional command
   edits. Update the compatibility page, `AGENTS.md`, and `CHANGELOG.md` to reflect
   only verified behavior; reconcile upstream drafts before filing them.
5. Validate each command in disposable fixtures: actual V1 with loaded plugins;
   V2 without plugins and with bounded native execution; neither backend usable;
   partial/incompatible plugin contracts; runtime failures and resume. Test that
   complete plugin capabilities select plugins regardless of reported version.
   Mocked selection tests do not establish real runtime compatibility. Capture
   GitHub mutations with controlled fixtures, never real PR writes.
6. Run strict OpenSpec validation, the approved guard and shell tests, applicable
   CI-derived local checks, and the required pre-PR council. Report unavailable
   runtimes or tooling as blockers/unverified rather than passed acceptance.

Rollback: revert the containment commits on the feature branch. The commands
return to the unmodified v0.21.0 scaffold and the known V2 limitation; plugin
sources, policy, and provider behavior are untouched.
