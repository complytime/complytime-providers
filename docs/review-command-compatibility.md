# Review command compatibility evidence

This page records the measured runtime status, the safeguard reconciliation, and
the bounded native reviewer prototype for the
`restore-plugin-free-review-commands` change in `openspec/changes/`. It is
repository-owned evidence, not part of the provisioned AI tooling guide.

Status: the three commands are a repository-owned native fork of the UF v0.21.0
scaffold (`make verify-uf-fork`). The prototype evidence below establishes
API-level reviewer lifecycle behaviour; it does not establish that each command
honours every safeguard end to end. Command-level verification remains open in
the change's tasks.

## Baseline and supported scope

The recovery targets `/uf.review-council`, `/uf.review-pr`, and
`/uf.address-feedback` only. On OpenCode v2.2.7 (the version measured when the
fork was created) and v2.3.0 (the version measured during the smoke checks) all
four UF plugins fail to load with `PluginModule.LoadError`, so no plugin tool is
registered and the three commands execute their native path. The capability
preflight also probes for native delegation: a runtime with neither the plugin
tools nor a delegation capability reports `UNAVAILABLE` and stops rather than
improvising a parent-only review. Other UF commands
and embedded `/uf.unleash` paths are outside scope. Preserve their plugins and
registrations.

Use **UF v0.19.0** as the command-structure baseline because it is the last
upstream release whose commands delegate natively; v0.19.1 introduced the plugin
layer. That tag resolves to commit
[`6b2dcd436d20e7f7663e8333ce82bcdf91dddeb7`][baseline], and its templates live at
`internal/scaffold/assets/opencode/commands/` in the `unbound-force` repository
(not in this one). The local templates were scaffolded from UF v0.21.0 and are now
forked, so reconcile against v0.19.0 for structure and against v0.21.0 for current
content. Do not force a scaffold downgrade, and do not run `uf init --force` while
the fork is active. The fork marker and `.uf/fork-manifest.sha256` identify the
owned files.

The baseline council delegates natively but mandates TodoWrite. Its PR command
uses one general reviewer, and feedback escalates directly to domain personas.
Do not copy these behaviors wholesale. Retain the current council authority,
selection policy, mutation gates, iteration limits, and partial-success verdicts;
native todos will be optional presentation, not recovery state.

Generic compatibility guidance belongs in `org-infra/docs/AI_TOOLING.md`, the
canonical source for the provisioned
[AI tooling guide](https://github.com/complytime/org-infra/blob/main/docs/AI_TOOLING.md).
That draft is not committed or published. Keep this repository's inventory and
runtime evidence here rather than in the provisioned guide.

### Current dependency inventory

This inventory includes shared skills, child prompts, feedback's post-fix council,
and resume paths. A forbidden alternative is not an additional required call.

| Surface | Native or external prerequisites | Current required UF plugin tools |
| --- | --- | --- |
| Council: input and scope | File discovery, Git, `gh` for explicit PRs | `capture_review_diff`; internally `resolve_base_ref` |
| Council: execution | Reviewer definitions, policy files, native delegation in the baseline | `plan_review_dispatch`, `dispatch_review_runs`, `consolidate_dispatch` |
| PR review | Git, authenticated `gh`, shell, private immutable diff | `plan_review_dispatch`, `dispatch_review_runs`, `consolidate_dispatch`, `finalize_review_dispatch` |
| Feedback: ingestion | Git, authenticated `gh`, pagination, private cache | `fetch_pr_review_context` |
| Feedback: Tier 2 | Reviewer definitions, policies, frozen feedback signal | `plan_review_dispatch`, `dispatch_review_runs`, `consolidate_dispatch`; the closing guardrail also mandates `dispatch_agent_run` |
| Feedback: execution | Question tool, Git, `gh`, cumulative post-fix council | `normalize_model_name`; council dependencies above |
| Council and feedback recovery | Native file read/edit; optional native todos in the intended recovery | `get_progress`, `checkpoint`, `clear_progress`; current templates also mandate TodoWrite |
| Child submission | Structured reviewer returns | `submit_review_findings`; optional `submit_lesson_proposal` |

The existing `pre-flight` and `review-context` skills use native file/shell
operations and Git/`gh`; they need no rollback. `dispatch-advisor` mandates plugin
planning and execution, so isolate the native instructions rather than changing
that skill for other consumers. The six review persona definitions themselves
require no plugin submission call; dispatch injects that requirement. The
council's `/uf.unleash` exit uses `format_unleash_exit` only when embedded, which
is outside this standalone recovery.

### Safeguard reconciliation

This table records source inspection on 2026-11-12, **not runtime equivalence**.
Read policy from its existing source on every invocation. Do not introduce a
second policy file or edit the sources to make the native path pass. Reconciliation
remains incomplete until the integration checks establish equivalent behavior.

| Safeguard and source | Native equivalent and acceptance check |
| --- | --- |
| Eligibility: `.uf/reviewer-capabilities.yaml`; dispatch `relevance` | Validate the manifest and sorted discovered files; content personas never vote. Retain Adversary/Guard, Curator relevance, and other scope intersections. Unknown discovered personas without manifest entries fail closed; absent known personas remain informational. |
| Change profile and tier caps: dispatch `classifyPath`, `profileDiff`, `diffTier`, `limitsFor`, `applyTierCap` | Derive categories and size from raw frozen diff statistics, not prompt heuristics. Preserve the inherited lightweight two-persona cap and uncapped standard/heavy defaults. Cap priority is required roles, alphabetical scope matches, then Curator; never drop a required role. Caps apply to default code/spec reviews, not full panels or feedback. Test boundaries and recorded skips. |
| Default/full runs: dispatch `baseRuns`, `fullPanelRuns`, `deduplicateRuns` | Preserve explicit run declaration order, then advisor profiles (`always` before mode default), then host fallback. `--full` forces one standard-profile run per eligible discovered reviewer, bypassing explicit run arrays and tier caps. Augmentation remains off for these commands. Curator has at most one run. |
| Models/variants: `.uf/review-matrix.yaml`, optional ignored override; executor `resolveHostModel` | Only override existing `profiles.*.model`/`variant` leaves. Invalid or unreadable overrides fail closed. Resolve unspecified identities from the active caller turn, not global defaults; compare child runtime identity and record requested/resolved/reported values separately. Caller-to-child resolution passes with fixture-supplied context; ordinary V1 caller discovery remains unresolved. |
| Limits: matrix plus dispatch defaults | Validate all planned persona/run counts before launch; never truncate a plan to satisfy a limit. Respect batch concurrency and report skipped/failed runs. Test configured and inherited limits without modifying policy. |
| Deadline: matrix `per_run_timeout_seconds`; executor lifecycle | Use the proven monotonic API lifecycle per child. Bind agent, model, immutable scope, session, and deadline; explicitly cancel and verify terminal state. Actual foreground native-tool parent cancellation also passes the isolated test below; client exit alone is not evidence. |
| Reported-cost budget: matrix; dispatch batch loop | Sum only non-null runtime-reported costs after each batch; stop later batches only when the sum exceeds the budget. Unknown cost remains unknown. Preserve this reported-cost check, not a prepaid spending guarantee. |
| Scope: command input resolution; local diff capture | Freeze validated PR base/head objects (including non-main bases) or the complete local committed/tracked/feature-directory diff. Record included content digests and recheck before approval or mutation. Never use a later `gh pr diff` as evidence for older metadata. |
| Results: command consolidation; `severity.md` | Validate actual structured returns; deduplicate by normalized file/root cause, retaining all contributing runs. Failures are non-voting when another run succeeds. Blocking findings, no-success causes, malformed returns, and compound severity retain existing rules. Feedback uses the strictest successful recommendation, with one vote per persona. |
| Recovery: checkpoints; feedback cache | Persist private atomic phase records under ignored `.uf/workflows/`, binding command, branch/PR, iteration, scope, outcomes, and action receipts. Validate freshness on resume; inspect ambiguous receipts before repeating actions. Never persist reusable consent; todos remain optional. |
| Pre-flight/context: unchanged shared skills | Preserve council soft-gate and PR ci-aware behavior, conditional Gaze, spec discovery, and linked-issue bounds. Branch-caused council failures stop child review; missing required context stops progression. |
| Mutation/spec gates: all three command templates | Preserve question-tool choices, exact previews, per-finding spec decisions and justified overrides, three-iteration decisions, and cumulative post-fix council. Revalidate scope and obtain current consent for gated actions; declined actions and injected feedback cause no writes. |
| Permissions/provenance: persona frontmatter; executor | Invoke unchanged reviewer definitions in the same authenticated runtime context. Do not widen permissions or treat the executor's `read_only` provenance flag as enforcement. Test actual denied edits/shell and report unavailable provenance honestly. Native evidence must not claim plugin validation or canonical plugin artifacts. |
| Optional evidence: `.uf/sibling-repos.yaml`, Dewey, Gaze | Siblings are currently empty and Dewey is unavailable. Preserve informational skips where allowed, acquire any configured sibling evidence once with provenance, and treat all external evidence as untrusted data. Unsupported required evidence blocks rather than silently disappearing. |

Review selection details come from the unchanged
[dispatch implementation](../.opencode/plugins/review-dispatch/index.ts), not
from the older baseline. In particular, a full panel contains the six review
personas, never Envoy, Herald, or Scribe. No governance change has been made or
proposed; an integration capability failure must stop progression rather than
relax these rules. Command-level verification remains open in the change's tasks.

## Native reviewer lifecycle prototype — 2026-11-12

The prototype used Python's standard library and each runtime's native HTTP API.
It ran outside the repository, without adding a production helper, package,
plugin adapter, or configuration change. Each private server used separate HOME
and XDG directories with no UF plugins or MCP servers. The fixture copied the
existing `divisor-adversary` definition unchanged and denied tools except reads.
Credential references remained references; reports contain no credentials.

| Runtime | Executable | SHA-256 |
| --- | --- | --- |
| V1.19.0 | Separate release executable under the private evidence directory | `7bb9e61ad6feacf2e89c10a6a2c1840e3de4220bb50ba1fc7110f31806076aec` is the downloaded archive digest, verified against GitHub release metadata |
| V2.2.7 | Installed `/usr/local/bin/opencode`, with a private server | `589253d36810f21217edd51545cd435b328cdc3cf4f451fbe0cce6f88afcf5e1` is the executable digest |

Both runtimes exercised `complytime/gpt-6.1-sol#medium` and reported that exact
model and variant. This was an explicit fixture selection, not proof of automatic
inheritance from the caller. A separate native V2 inspection resolved the live
caller's session ID from `OPENCODE_SESSION_ID` and retrieved its model through
`opencode api`; its actual variant was `default`, not the global configuration's
`medium`. Integration must use runtime identity rather than assume config parity.

The immutable fixture changed an admin check to unconditional access. Normal
completion required valid JSON reporting `REQUEST CHANGES` and a HIGH finding at
`access.go:2`. Cancellation fixtures requested a longer assessment of that same
change, so an actual unfinished reviewer was active at cancellation time.

| Scenario | V1.19.0 | V2.2.7 |
| --- | --- | --- |
| Structured review completes normally | PASS, 6.604 s | PASS, 5.341 s |
| Monotonic five-second deadline cancels reviewer | PASS, 5.020 s | PASS, 5.029 s |
| Supervisor receives actual SIGTERM | PASS, 5.015 s | PASS, 5.031 s |
| False affirmative cancellation acknowledgement | PASS: rejected as `INCONCLUSIVE` | PASS: rejected as `INCONCLUSIVE` |
| Fresh controller recovers saved identity and cancels | PASS, 5.240 s | PASS, 5.084 s |

Five seconds is a shortened test deadline, not a policy change. The repository's
configured deadline remains 1,200 seconds. Elapsed values include cancellation
and terminal-state verification; they are observations, not latency guarantees.

Cancellation used V1's `POST /session/{sessionID}/abort` or V2's
`POST /api/session/{sessionID}/interrupt`, discovered against exact running
schemas. Before cancellation, assertions required an active unfinished assistant
with the expected model. After cancellation, V1 had idle status and a completed
assistant carrying `MessageAbortedError`; V2 had outcome `interrupted`, no active
execution, no unfinished assistant, and empty job and inbox lists. No result was
accepted from an acknowledgement or a terminated client alone.

The failed-cancellation fixture deliberately returned an affirmative adapter
response without invoking cancellation. The still-active runtime caused rejection;
only then did fixture cleanup perform and verify real cancellation. Cleanup did
not change that attempt's `INCONCLUSIVE` result.

Recovery launched a separate controller process while the private server stayed
alive. It read the saved session identity, model, scope digest, monotonic deadline,
and boot identity, validated them, and cancelled the same reviewer. This tests
controller replacement, not full command recovery after arbitrary crashes.

## Native shell boundary and caller-model resolution — 2026-11-12

A second disposable probe tested the missing parent-tool boundary. An actual
model-driven controller invoked a foreground native shell tool that ran a
Python supervisor. That supervisor created an actual `divisor-adversary` child
in the same private server. The outer test required an active unfinished
reviewer with the expected model and a real shell call in the parent transcript
before interrupting the parent through its native API.

Both runtimes delivered SIGTERM to the supervisor. Its signal handler cancelled
the reviewer and verified terminal state, using the same V1/V2 criteria as the
standalone suite. Cleanup outside the supervisor did not supply the passing
cancellation evidence. The controller alone had fixture shell permission; the
unchanged copied reviewer definition and additional child-session restrictions
kept the reviewer read-only. This does not prove all production permissions.

The final cases also resolved the active parent assistant's model through the
API before selecting the child model. V2's private global configuration selected
`#default`, while the actual parent and child both reported `#medium`; the
supervisor used the parent, not the global default. V1 likewise propagated its
observed parent model and variant into the child. The parent session ID and
server URL were supplied explicitly by the fixture driver in both cases.

| Final case | V1.19.0 | V2.2.7 |
| --- | --- | --- |
| Native parent interruption reaches supervisor | PASS, SIGTERM received | PASS, SIGTERM received |
| Child cancellation independently verified | PASS, 0.055 s after parent cancellation | PASS, 0.022 s after parent cancellation |
| Parent-to-child runtime identity | `complytime/gpt-6.1-sol#medium` | `complytime/gpt-6.1-sol#medium`, despite global `#default` |
| Native shell exposes `OPENCODE_SESSION_ID` | No | Yes |

An earlier explicit-model-only boundary test also passed (V1: 0.092 s;
V2: 0.022 s). These elapsed times describe local observations, not cleanup
latency guarantees. All private servers stopped in finalizers. No command
templates, policy files, plugins, installed runtimes, or global settings changed.

The stdlib-only probe and private reports were written during that session to
`/tmp/opencode/review-native-tool-boundary-20261112-iOBNq2/`; those paths are
session-local scratch, not retained evidence; final caller-resolution
reports live in its `caller-inheritance/v1/` and `caller-inheritance/v2/`
subdirectories. Do not publish private configuration or transcripts
without checking for sensitive data.

### V1 automatic-context investigation

The fixture-supplied server URL and parent session identity are not proof that
the standalone commands can discover their caller. V1's actual native shell did
not expose `OPENCODE_SESSION_ID`, and V1.19.0 has no `opencode api` command. Its
pinned [shell implementation][v1-shell] inherits the server environment and does
not inject the caller ID. V2 injects that ID and provides authenticated API
discovery through its CLI.

Further investigation found a transport limitation, not only a missing environment
variable. In V1.19.0's pinned [TUI startup][v1-tui], ordinary startup uses
`http://opencode.internal` with a worker-backed fetch function. Only an explicit
network option requests `Server.listen`. Its [worker][v1-worker] forwards requests
directly to the in-process application; [worker RPC][v1-rpc] uses `postMessage`,
not a published CLI/API endpoint. The [local run implementation][v1-run] likewise
supplies an in-process fetch function. There is no default HTTP endpoint for an
external supervisor to discover in those paths.

An ordinary, noninteractive `run --command` fixture verified this without `serve`,
`attach`, an injected caller ID, or an injected server URL. The actual model
executed a native foreground shell tool. Both its shell receipt and the command's
pre-expansion shell receipt reported no `OPENCODE_SESSION_ID`; inspection limited
to their own OpenCode ancestor found zero TCP listeners. The literal
`$SESSION_ID` remained unexpanded in the submitted command. Runtime identity was
available to the outer CLI driver in JSON events, not automatically handed to
the command's shell process.

| Candidate | V1.19.0 finding |
| --- | --- |
| Shell environment | No caller ID or endpoint injected. `OPENCODE_PID` identifies a process only; pinned startup sets it from `process.pid`. |
| Command substitution | Only arguments/positions and shell-output expansion supply values; no session-context placeholder was found. The fixture confirmed `$SESSION_ID` stays literal. |
| Native CLI diagnostics | `debug info` reports version, OS, terminal, and plugins, not current caller/endpoint. V1 has no `opencode api` command. |
| Session list/export | Listing is not a current-caller binding. Export with no ID prompts interactively; exact-ID export requires the identity already known. The fixture exported only its own exact ID through the official CLI, never direct database access. |
| Native Code Mode | The pinned implementation exposes connected MCP tools; it does not provide V2's built-in `tools.opencode` session-management namespace. Do not infer V1 tools from the current V2 session's catalog. |
| Native Task | Internal context knows the current assistant model and caller, but its public inputs expose neither arbitrary model selection nor a deadline. It cannot replace policy-preserving bounded execution by itself. |
| Default TUI/local run transport | In-process application fetch, not an external HTTP listener. Starting a second server does not give the caller's active execution or cancellation context. |

The accepted fixture report is at
`/tmp/opencode/review-v1-context-discovery-20261112-rr8eyi/report.json` alongside
the stdlib-only probe, receipts, private export, and pinned source; that path is
session-local scratch, not retained evidence.
Initial fixture attempts inherited the harness's stale `PWD`, loaded the wrong
project context, and failed before the probe ran; those attempts were rejected.
The accepted run sets both subprocess working directory and `PWD` to the isolated
fixture, matching an ordinary shell launch. It loads the private configuration
with external plugins disabled. It is not an interactive TUI test; TUI transport
findings above are source inspection.

**Conclusion:** no supported automatic CLI/API context handoff was found for
default V1.19.0 under the approved constraints. The bounded supervisor's HTTP
path cannot attach where no listener exists. Do not guess the latest session,
scan unrelated processes, inspect private databases, introduce a plugin, or
pretend a new server is the existing caller. This is a finding about the exact
tested version and interfaces, not proof against every possible runtime design.

An explicit V1 `serve`/`attach` launch with trustworthy server/session context
remains a possible small integration path without new packages or global settings.
It adds a contributor prerequisite and has not been approved. An upstream native
context/transport interface is another possible resolution, not implemented here.
Command-level verification stays incomplete, and no cross-version command
compatibility is claimed; only the native path on the recorded V2 version is
exercised.

## Command smoke observations — 2026-11-16

Executed on OpenCode v2.3.0 with no UF plugin tools registered. Read-only: no
GitHub writes, no commits, and no feedback fixes.

```text
Execution path: native (UF plugins unavailable on OpenCode v2.3.0)
Protocol (prompt-enforced): frozen review scope, council participation, mutation gates
Runtime-enforced: none
Not provided: plugin dispatch artifacts, plugin progress checkpoints
```

### PR review smoke — PR #239

- Scope: 35 product, 0 governance, 0 dependency, 0 excludable tooling files. The
  frozen inventory is `go.mod`, `go.sum`, and vendored packages.
- Result: `INCONCLUSIVE`. The executing session exposed no delegation tool, so no
  reviewer was dispatched and no verdict was produced. This run is why the
  preflight now probes for native delegation and reports `unavailable` instead of
  claiming the native path.
- Retained: 139 KB for this invocation. The tree total was 17.6 MB, dominated by
  one prior 17.3 MB invocation.

### Feedback triage smoke — PR #239

- Scope: the same 35 dependency files; no tooling paths.
- Ingest: two approval reviews filtered, zero inline threads, four issue comments.
  One actionable maintainer item and three informational bot reports survived.
- Assessment: no security-relevant item, so no Tier 2 run was made.
- Digests: `scope_digest` `a9c4be7c…`; per-item digests were written to
  `.uf/feedback/pr-239/state.json`.
- Retained: 308 KB for this invocation; the prior tree was 16.2 MB.

### What these runs do not establish

They exercised preflight, ingestion, scope classification, digest recording, and
the `unavailable` stop. They did not produce a reviewer verdict or a
mutation-gate decision, and they make no cross-version claim. The council runs are
recorded separately in the change's tasks.

## Limits and retained evidence

- These are API lifecycle and native-tool boundary tests, not runs of the three
  repaired commands. Ordinary caller discovery, default/full selection, policy
  overrides, production permissions, budgets, scope freshness, and mutation gates
  still need integration checks. The command smoke runs are recorded in tasks
  7.3-7.4; they do not convert these API-level results into full command
  verification.
- An earlier V2 two-second fixture did not establish an active reviewer and was
  inconclusive. The final suites use five-second cancellation fixtures and assert
  active reviewer identity before cancellation.
- One V1 private server teardown required SIGKILL after its reviewer was already
  verified inactive. Server termination is not counted as cancellation evidence.
- Runtime-reported cost is not a pricing guarantee: V1 reported zero for the normal
  review. Neither unknown nor zero cost should be presented as proof of free use.
- No external PR mutations occurred. Installed runtime files, global settings,
  persona definitions and plugin registrations were not changed. No governance
  rule or threshold was modified; governance-defining files gained explicit
  always-in-scope classification in the shared contract.

Local scripts, private runtime data, exact schemas, and per-case JSON reports were
written to `/tmp/opencode/review-deadline-native-20261112-LhZsqc/`. Those paths are
session-local scratch space, not retained evidence: no later reader can verify
them, so the claims that rest on them are observations from that session rather
than reproducible artifacts. The suites ran as `python3 probe.py v1 suite` and
`python3 probe.py v2 suite` from that directory.

See the safeguard mapping in
`openspec/changes/restore-plugin-free-review-commands/design.md` (section
"Safeguard mapping") for the remaining integration requirements. It is cited as a
path rather than a link because the change directory moves to `openspec/archive/`
when the change is archived.

Local checks for the containment implementation passed: `make build`,
`make test`, `make verify-uf-fork`, the guard test
(`bash .github/scripts/test/verify_uf_fork_test.sh`), `openspec validate
--strict`, and `git diff --check`. `make test` runs Go tests only; the guard test
is a separate command and is not wired into a workflow. Full MegaLinter and
cross-repository CI were not run; `markdownlint-cli2` is unavailable.
`README.md` was assessed and needs no change; `AGENTS.md` gained the local-fork
section and the verification targets, and `CHANGELOG.md` records the fork and the
evidence page. Command-level runtime verification is recorded in tasks 7.3-7.4; the pre-PR
council is task 7.2.

## Isolated command verification procedure

1. Keep V1.19.0 at the separate executable path recorded above and use the
   installed V2.2.7 executable without replacing either. Verify versions and
   digests before testing; use each exact runtime's served API schema.
2. Create disposable Git repositories with separate HOME and XDG config, data,
   cache, and state directories. Preserve credential references without printing
   secrets. Load only the selected commands, their required shared instructions,
   and unchanged reviewer definitions; make UF plugins unavailable. Do not edit
   global settings or enable experimental todos.
3. Exercise code/spec council modes, default/full selection, overrides, limits,
   budgets, permissions, structured successes and failures, native-tool parent
   cancellation, scope changes, and recovery. Record actual runtime identity,
   cancellation state, unknown costs, and every non-voting failure. Verify the
   complete dependency closure, not only initial command discovery.
4. Exercise PR review with a non-main base and feedback with accepted/rejected
   items using controlled `gh` fixtures. Capture mutation attempts without real
   PR writes. Decline posting/push/replies/resolution and verify no action occurs;
   normal feedback smoke checks stop before external writes. Stale scope and
   ambiguous action receipts must not authorize replay.
5. Record a per-command V1/V2 evidence matrix, exact versions, failures, and
   redacted observations. Until these runs pass, report command compatibility as
   unverified. Static checks and the lifecycle prototype cannot substitute for
   command execution or the required pre-PR council.

### Runtime investigation references

V1.19.0 resolves to `733b2c627e5aec39cb33cc22373bdce33053ef2c`; V2.2.7 resolves
to `fedc6f208aa2bd54c1019227214568083f6380ad`. The pinned [V1 Task][v1-task] and
[V2 subagent][v2-subagent] have no per-run deadline parameter. V1's
[abort handler][v1-abort] and V2's [noninteractive lifecycle][v2-lifecycle]
provide cancellation primitives; use the exact served schemas for requests.

An earlier shell-only V2 negative probe received `{"interrupted": false}` while
its bounded process continued. This was not a reviewer cancellation test and
does not contradict the passing reviewer lifecycle suite. The live shell schema
also differed from the documentation site's response code. Neither HTTP success
nor provider request timeouts proves that a complete reviewer run has stopped.
Do not enable experimental schedules as a replacement deadline mechanism.

[baseline]: https://github.com/unbound-force/unbound-force/commit/6b2dcd436d20e7f7663e8333ce82bcdf91dddeb7
[v1-task]: https://github.com/anomalyco/opencode/blob/733b2c627e5aec39cb33cc22373bdce33053ef2c/packages/opencode/src/tool/task.ts
[v1-shell]: https://github.com/anomalyco/opencode/blob/733b2c627e5aec39cb33cc22373bdce33053ef2c/packages/opencode/src/tool/shell.ts
[v1-tui]: https://github.com/anomalyco/opencode/blob/733b2c627e5aec39cb33cc22373bdce33053ef2c/packages/opencode/src/cli/cmd/tui.ts
[v1-worker]: https://github.com/anomalyco/opencode/blob/733b2c627e5aec39cb33cc22373bdce33053ef2c/packages/opencode/src/cli/tui/worker.ts
[v1-rpc]: https://github.com/anomalyco/opencode/blob/733b2c627e5aec39cb33cc22373bdce33053ef2c/packages/opencode/src/util/rpc.ts
[v1-run]: https://github.com/anomalyco/opencode/blob/733b2c627e5aec39cb33cc22373bdce33053ef2c/packages/opencode/src/cli/cmd/run.ts
[v1-abort]: https://github.com/anomalyco/opencode/blob/733b2c627e5aec39cb33cc22373bdce33053ef2c/packages/opencode/src/server/routes/instance/httpapi/handlers/session.ts
[v2-subagent]: https://github.com/anomalyco/opencode/blob/fedc6f208aa2bd54c1019227214568083f6380ad/packages/core/src/tool/plugin/subagent.ts
[v2-lifecycle]: https://github.com/anomalyco/opencode/blob/fedc6f208aa2bd54c1019227214568083f6380ad/packages/cli/src/run/noninteractive.ts
