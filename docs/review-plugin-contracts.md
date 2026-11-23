# Pinned review plugin contracts

This inventory supports task 8.1 of `restore-plugin-free-review-commands`: use it
to check the complete plugin dependency closure before restoring the three
review workflows. It describes inspected UF v0.21.0 sources, not implemented
runtime compatibility. Dual-path command instructions are being implemented;
their end-to-end acceptance remains pending. In particular, the pinned feedback
contract has the incompatibilities recorded below; tool registration alone must
not select that path.

## Source and inspection boundary

UF v0.21.0 is an annotated tag whose target commit is
`ebed75730066865d777288a999aa3906f6a8b3f7`. References below point to that immutable
commit's scaffold assets, not upstream's working `.opencode/` directory:

- [Council template][council], especially input resolution, the shared dispatch
  protocol, checkpoint tracking, and the posting gate.
- [PR template][pr], especially sections 4b–5b.
- [Feedback template][feedback], especially sections 1.4, 2.3, 4.2–4.3, and guardrail 11.
- [Dispatch advisor][advisor], including the prohibition on individual run dispatch.
- [Review plugin][dispatch], its registered tools, schemas, injected child response
  contract, aggregate execution, consolidation, and recovery handlers.
- [Workflow plugin][workflow], its progress and attribution tool factories.
- [Diff capture implementation][diff], including its internal base resolution.

The `invoke-agent` and `uf-finale` plugin entrypoints were also inspected. Neither
is a direct workflow prerequisite for these three standalone commands. Embedded
`/uf.unleash` behavior, including `format_unleash_exit`, remains outside scope.

## Required closure by command

“Required” means the tool must be available before selecting plugins, not that
every phase always calls it. Resolve conditional requirements from explicit
arguments and enabled workflow phases without fetching feedback or capturing a
diff. When a condition is not yet knowable, include its possible dependencies;
do not select a partial plugin path and discover a missing tool after execution.

| Command | Required parent calls | Conditional and transitive dependencies |
| --- | --- | --- |
| Council | `plan_review_dispatch`, `dispatch_review_runs`, `consolidate_dispatch`, `get_progress`, `checkpoint`, `clear_progress` | Local input requires `capture_review_diff`; explicit PR input uses Git/`gh` instead. Child sessions need `submit_review_findings`. Finalization is internal to provenance-bearing consolidation, not a separate parent call. |
| PR review | `plan_review_dispatch`, `dispatch_review_runs`, `consolidate_dispatch`, `finalize_review_dispatch` | The pinned template captures the immutable PR diff through Git/`gh`, not `fetch_pr_review_context`. Child sessions need `submit_review_findings`. Unlike council and feedback, this template does not mandate plugin checkpoints. |
| Feedback | `fetch_pr_review_context`, `get_progress`, `checkpoint`, `clear_progress` | Possible Tier 2 items require planning, aggregate dispatch, consolidation, and child findings submission. Enabled fixes require `normalize_model_name` and the full cumulative post-fix council closure, even when initial items are Tier 1. |

The revised command instructions explicitly add the progress trio to PR plugin
preflight rather than claiming it was present in the pinned template. They also
require `dispatch_status` for each command's correlation binding and recovery.
All resumed plugin dispatches need explicit-correlation consolidation. Verify
these recovery contracts before selecting a path that promises recovery, not
only when a restart happens.

`dispatch_review_runs` internally acquires sibling evidence, builds prompts,
executes planned children, and persists run/session data. Its internal
`dispatchAgentRun` call is not a requirement to invoke the separately registered
`dispatch_agent_run` tool. Likewise, internal base resolution and finalization
do not require separate parent calls to `resolve_base_ref` or
`finalize_review_dispatch` for council/feedback. Inspect those implementations
as part of aggregate compatibility rather than probing them through execution.

## Contract checks

Compare the exposed tool schema and trusted implementation contract to the
pinned source, including types, required fields, enum values, validation bounds,
output visibility, and side effects. A matching name or an accepted argument
shape alone does not establish compatible behavior.

| Tool | Inputs and observable semantics that must be preserved |
| --- | --- |
| `plan_review_dispatch` | Accepts mode, all discovered agents, `full`, `augment`, and raw `{path, additions, deletions}` records. Returns the versioned plan, limits, entries, errors, and workflow result. Only `ready` with null workflow result and no errors permits dispatch. No native reinterpretation of rejected policy. |
| `dispatch_review_runs` | Accepts frozen diff path, changed-path text, serialized input context, unchanged plan entries, session metadata, parallel limit, budget, timeout in milliseconds, and optional context. Session metadata includes command, mode, full, structured input context, change profile, plan, and coverage. The model-visible result is a counts summary, not the findings/proposals arrays in metadata. |
| `submit_review_findings` | Available to the actual child persona under its existing permissions. Accepts findings with severity, category, description, root cause, nullable file, and nullable line. Submissions are associated with the child session and harvested into the correct run; parent text is not submission evidence. The pinned array bound is 1–512. |
| `consolidate_dispatch` | Optional UUID `correlation_id` permits recovery from a new parent session. Without provenance, retrieves persisted results and validated proposals without finalizing. With provenance, finalizes internally and returns authoritative artifact, calculated assessment, and canonical verdict data. Errors and `not_found` must not become approval. |
| `finalize_review_dispatch` | PR review supplies the closed terminal payload plus provenance. Validation and atomic persistence determine whether the returned assessment is authoritative; persistence failure cannot authorize automation. Council/feedback must not substitute this call for their internal finalization protocol. |
| `capture_review_diff` | Optional `feature_dir`; returns bounded diff path, base information, mode, fallback reason, and per-file counts. Captures the intended local tracked and feature-directory content. Validation, Git, size, or I/O errors stop review. |
| `fetch_pr_review_context` | Positive PR number; returns structured PR context and a saved diff path or a structured error. Fetch once for feedback; use its diff outputs without replacing the template's paginated authoritative feedback ingestion. |
| `get_progress` | Command and optional key; returns the full saved state or null in a success envelope, or an explicit error envelope. Default key derives from the branch. A prior checkpoint alone does not prove path ownership. |
| `checkpoint` | Command, step, optional key, and optional `data` as a **JSON object string**, not a raw object. Shallow-merges data and persists completed progress. A failed write is not phase completion. |
| `clear_progress` | Command and optional key; deletes completed workflow progress idempotently. Never use it as a capability test or before completion and receipt reconciliation. |
| `normalize_model_name` | `rawModelId` string; returns an envelope containing `normalized`, with a warning for unknown identity. Preserve the warning behavior; never invent model provenance. |
| `dispatch_status` | Optional explicit correlation UUID; reads persisted run statuses, counts, usage, and pending-submission count. Returns `not_found` for absent state. It neither cancels active children nor proves their terminal state merely by being registered. |

Check child visibility separately from parent registration. Check aggregate
model/variant resolution, inherited persona permissions, timeouts, cancellation,
terminal-state observation, policy limits, and provenance through trusted source
inspection and later controlled execution. The probe itself cannot prove these
runtime properties.

## Optional evidence and recovery operations

- `submit_lesson_proposal` is optional child learning submission; Dewey storage is
  optional parent work. Missing either must produce an informational skip, not
  force native selection. Preserve any successfully validated proposals.
- Gaze is optional when unavailable. If present, the council template delegates
  to `gaze-reporter`; inspect that enabled delegation separately from reviewer
  dispatch. Failure is not permission to fabricate quality evidence.
- Sibling acquisition belongs inside aggregate dispatch. Preserve configured
  evidence policy and report unavailable evidence; do not require a separate
  registered `acquire_sibling_evidence` call merely because the aggregate uses
  its implementation.
- `redispatch_failed_runs` and `discard_orphaned_submissions` are registered
  recovery operations, not mandated normal template phases. If a recovery
  procedure enables either, include its schema/ownership checks before that
  procedure. Redispatch requires the saved correlation, preserves the original
  plan/prompt and provenance, and refuses successful runs. Neither operation
  permits backend switching, successful-assessment replay, or altered verdicts.
- `build_review_prompts`, `read_assessment_fields`, `prepare_lesson_learning`,
  `invoke_agent`, and finale tools are not required direct calls in these
  standalone templates. Do not substitute them for required aggregate calls.
- The pinned council and feedback templates mandate TodoWrite for presentation.
  The revised native design makes todos optional; presentation never replaces
  persistent backend progress.

## Side-effect-free capability probe

This inspection procedure uses a pure `jq` decision helper for tasks 8.3 and 8.6,
not a new runtime adapter. The helper is consumed by the shared command preflight,
so its fixture tests exercise the same decision logic rather than a test-only
reimplementation.

1. Parse the command and explicit phase options; read the existing local binding
   and policy/agent definitions without modifying them. Preserve any saved
   backend ownership. Compute the complete required closure above, including
   possible later feedback phases and recovery.
2. Enumerate the current session's registered tools and inspect exposed schemas
   and trusted contract/source references. Record exact callable identifiers,
   parent/child visibility, and evidence for behavioral compatibility. Discovery
   may retrieve tool definitions; it must not execute workflow tools.
3. Classify each required capability as compatible, missing, incompatible, or
   unknown. Only an authoritative complete enumeration establishes absence.
   Ambiguous discovery, hidden child visibility, or unverifiable behavioral
   contracts is unknown, not “plugins unavailable”; stop for reconciliation.
4. A complete compatible plugin closure wins regardless of reported OpenCode
   version or native delegation availability. Missing/incompatible plugin
   contracts permit evaluating the complete bounded native controls; report the
   specific gaps. Optional evidence absence does not disqualify plugins.
5. Inspect native delegation, model/variant, permissions, deadline, cancellation,
   and terminal-state controls only if needed. If neither path is complete,
   report `UNAVAILABLE`. Runtime version is diagnostic metadata, never selection
   evidence.
6. Report one truthful banner, then persist the selected-path binding before any
   backend operation. Resume must retain the saved path; lost capabilities stop
   recovery rather than authorizing fallback. No reviewers, checkpoints,
   ingestion artifacts, PR writes, or other workflow mutations occur during the
   probe itself.

Fixtures must distinguish absence from uncertainty, parent registration from
child visibility, and schema compatibility from runtime evidence. Real V1 and V2
command execution remains task 8.8; this inventory does not satisfy it.

### Executable decision inputs

`.github/scripts/uf-review-preflight.jq` reads one JSON object on stdin through
`jq -L .github/scripts -cs -f`. It performs no discovery, launches no process,
and writes no records. Both filters include the shared format definitions in
`.github/scripts/uf-review-common.jq`.
Use these fields after inspecting actual tools and trusted source contracts:

| Field | Required value |
| --- | --- |
| `command` | One of the three complete `uf.*` command identifiers |
| `local_input` | Boolean: local council input; feedback also includes local post-fix council capture |
| `siblings_configured` | Boolean derived from the unchanged sibling configuration |
| `runtime_version` | Actual version, or explicit unknown; diagnostic only |
| `discovery` | `complete` only with authoritative inventory; otherwise `ambiguous` |
| `plugin`, `native` | Capability maps with `{status, evidence}` records; evidence is a nonempty bounded source/reference, not a self-reported promise |
| `binding` | `{status, execution_path, unreconciled}`; status is `fresh`, `valid`, `legacy`, or `incompatible`; fresh has null path, valid has the inspected saved path |

Capability status is `compatible`, `missing`, `incompatible`, or `unknown`.
The helper owns the required closure listed in its `plugin_requirements`
function. Beyond registered tools, its semantic checks are `immutable_scope`,
`child_contract`, `lifecycle`, and `policy`; feedback adds `feedback_assessment`,
and configured siblings add `frozen_sibling_evidence`. These are inspected
behavioral contracts, not invented tool names. The pinned feedback implementation
must receive `incompatible` for `feedback_assessment`, with the source findings
below as evidence; an actually compatible future implementation may pass.

Native controls are `delegation`, `model_variant`, `permissions`, `deadline`,
`cancellation`, `terminal_state`, `caller_binding`, and `policy`. Plugin selection
does not depend on any native control. Missing capabilities under complete
enumeration are reported by name; unknown required contracts stop selection.
Optional tool absence does not invalidate either required closure.

The command, not the helper, verifies that a saved binding belongs to this
project/command/target and validates the referenced backend evidence before
marking `binding.status` valid. Any unreconciled child or ambiguous receipt sets
`unreconciled: true`. Schema errors fail closed. The helper never treats its own
result as evidence that these runtime inspections happened.

`.github/scripts/uf-review-recovery.jq` similarly consumes one inspected projection:

- `binding`: exact version-1 fields `execution_path`, `command`, `invocation_id`,
  canonical `project`, `target`, `backend_ref`, `scope_digest`, `policy_digest`,
  `agents_digest`, `active_children` (IDs), and `receipts` (`{id, status}` with
  `confirmed` or `ambiguous` status).
- `current`: command, project, target, execution path, scope/policy/agents digests,
  and the current `item_digest`. Digests are lowercase SHA256 strings.
- `assessment`: null, or execution path, backend reference, scope/item/policy/
  agents digests, and inspected `plugin-validated` or `native-validated`
  provenance matching its backend.
- `council`: null, or execution path, cumulative scope/policy/agents digests and
  actual verdict.
- `event`: `resume`, `runtime_error`, `policy_rejection`, `model_unavailable`,
  `blocking_verdict`, or `post_fix_council`.

Project/target/path mismatch, legacy schemas, ambiguous receipts, active children,
and incompatible provenance stop recovery. Changed assessment inputs request
reassessment with a superseded marker; matching evidence requests reuse.
Post-fix review is checked against the current cumulative scope and backend.
Every output has empty backend actions, false mutation authorization and a fresh
consent requirement. The command still validates actual artifacts/results and
performs any permitted actions through its existing gates.

Run `make test-uf-review` for isolation, selection, and controlled recovery
decision fixtures. These are not real plugin failures, reviewer lifecycle tests,
or full command execution; they cannot establish task 8.8 compatibility.

## Pinned inconsistencies requiring implementation care

The feedback closing guardrail names individual `dispatch_agent_run` execution,
but its detailed section 2.3.4 and the advisor require `dispatch_review_runs`.
The aggregate internally executes planned runs; do not turn the closing wording
into a second dispatch or require both public entrypoints.

The injected response contract requests findings submission, while its tool
rejects an empty findings array. A clean review must never invent a finding to
satisfy submission. Resolve the zero-findings interpretation in the restored
instructions and fixtures before claiming command compatibility.

The templates show object-shaped checkpoint examples, while the exposed
`checkpoint.data` schema requires serialized JSON. Restore calls against the
actual exposed schema, not the shorthand examples. None of these observations
authorizes modifying plugin sources, review policy, or the existing fork guard.

### Feedback restoration blocker

Source inspection during task 8.4 found a behavioral gap, not just a naming or
schema difference:

- The pinned review plugin's `buildReviewPrompts` tests only whether mode is
  `triage` (line 4305). At line 4498 it injects `RESPONSE_CONTRACT` for every other
  mode, including feedback. That contract requests code-review verdicts, not the
  feedback assessment fields required by the pinned feedback template.
- `review_context` is explicitly wrapped as untrusted data; putting replacement
  child instructions there cannot repair the trusted response contract.
- `workflowVerdictValue` at lines 3624–3634 maps code-review `APPROVE` to feedback
  `ACCEPT` and advisory/blocking results to `AUTHOR-DECIDES`. This is not evidence
  that a child assessed whether the current feedback item should be accepted.
- The aggregate calls sibling acquisition on every dispatch (line 4628) and has
  no input for a retained invocation-wide evidence bundle. For configured
  siblings, preserving one frozen acquisition across items/iterations needs a
  compatible aggregate contract, not a second parent acquisition.

The command preflight must report these as incompatibilities where applicable.
Do not change plugin sources or registrations, elevate untrusted context into
instructions, synthesize missing feedback fields, or switch backend after
dispatch. Restoring and accepting all three commands on actual V1 plugins remains
blocked for the pinned implementation. The maintainer elected to keep plugin
sources unchanged and continue the capability-based fallback: classify that
feedback contract as incompatible and select bounded native execution before
backend work when supported. This does not waive the remaining real-runtime
acceptance checks or claim that V1 plugin-backed feedback passed.

See [compatibility evidence](review-command-compatibility.md) for the native
observations and [the change tasks][tasks] for approval and acceptance work.

[council]: https://github.com/unbound-force/unbound-force/blob/ebed75730066865d777288a999aa3906f6a8b3f7/internal/scaffold/assets/opencode/commands/uf.review-council.md
[pr]: https://github.com/unbound-force/unbound-force/blob/ebed75730066865d777288a999aa3906f6a8b3f7/internal/scaffold/assets/opencode/commands/uf.review-pr.md
[feedback]: https://github.com/unbound-force/unbound-force/blob/ebed75730066865d777288a999aa3906f6a8b3f7/internal/scaffold/assets/opencode/commands/uf.address-feedback.md
[advisor]: https://github.com/unbound-force/unbound-force/blob/ebed75730066865d777288a999aa3906f6a8b3f7/internal/scaffold/assets/opencode/skills/dispatch-advisor/SKILL.md
[dispatch]: https://github.com/unbound-force/unbound-force/blob/ebed75730066865d777288a999aa3906f6a8b3f7/internal/scaffold/assets/opencode/plugins/review-dispatch/index.ts
[workflow]: https://github.com/unbound-force/unbound-force/blob/ebed75730066865d777288a999aa3906f6a8b3f7/internal/scaffold/assets/opencode/plugins/uf-workflow/index.ts
[diff]: https://github.com/unbound-force/unbound-force/blob/ebed75730066865d777288a999aa3906f6a8b3f7/internal/scaffold/assets/opencode/lib/review-diff.ts
[tasks]: ../openspec/changes/restore-plugin-free-review-commands/tasks.md
