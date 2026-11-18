# Spec Delta

## Purpose

Keep the three main review commands plugin-backed when the complete required UF
tool contract is available, and otherwise use a supported native fallback. Make
the active execution path, reviewed scope, and limits of the evidence explicit.

## ADDED Requirements

### Requirement: Execution path is always reported

The commands `/uf.review-council`, `/uf.review-pr`, and `/uf.address-feedback`
SHALL detect whether the complete required UF plugin tool contract is available
before backend work. They SHALL report one execution-path banner naming the path
taken and the safeguards it does and does not enforce. A command SHALL NOT reach
a verdict, a triage summary, or a gated mutation while the path is unreported.

#### Scenario: Plugin tools are unavailable

- **GIVEN** an OpenCode runtime where no UF plugin tool is registered and all
  required native controls are available
- **WHEN** a contributor invokes one of the three commands
- **THEN** the command prints the native-path banner before dispatch
- **AND** it names the plugin capabilities it will not provide

#### Scenario: Plugin tools are available

- **GIVEN** a runtime where the required UF plugin tools are registered
- **WHEN** a contributor invokes one of the three commands
- **THEN** the command reports the plugin path
- **AND** it executes the plugin-backed workflow rather than native orchestration
- **AND** it does not mix plugin and native execution within the invocation

#### Scenario: No review can be produced

- **WHEN** neither the plugin tools nor a required native capability is available
- **THEN** the command reports `UNAVAILABLE`, names the missing capability, and stops
- **AND** it does not produce a parent-only review as a substitute

#### Scenario: Native delegation is absent

- **GIVEN** a runtime with no plugin tools and no tool that can start a child agent
- **WHEN** the preflight runs
- **THEN** it prints the `unavailable` banner rather than the `native` banner
- **AND** no reviewer is dispatched and no verdict is produced

#### Scenario: Runtime version cannot be determined

- **WHEN** the runtime does not expose its version
- **THEN** the banner reports `vunknown (version not determinable)`
- **AND** no version is inferred or invented

### Requirement: Prefer a complete plugin contract before native execution

Each command SHALL select the plugin path when its complete required tools expose
compatible contracts in the current session. Selection SHALL inspect registered
capabilities, not infer availability from runtime version, configuration, or load
logs alone. The required set SHALL cover initial work, enabled conditional phases,
child submission, recovery, and post-fix council review. Optional capabilities
SHALL remain optional according to existing policy.

#### Scenario: Compatible plugins on OpenCode V1

- **GIVEN** a V1 runtime with the complete required plugin contracts registered
- **WHEN** any of the three commands starts
- **THEN** it selects the plugin path without requiring native supervisor context
- **AND** it uses plugin capture or ingestion, planning, dispatch, structured
  submission, checkpoints, and finalization as applicable to that command

#### Scenario: Future compatible plugins on OpenCode V2

- **GIVEN** a V2 runtime with the complete required plugin contracts registered
- **WHEN** preflight runs
- **THEN** it selects the plugin path regardless of the reported runtime version
- **AND** native delegation availability does not override that selection

### Requirement: Native fallback requires complete native capabilities

When required plugin contracts are missing or incompatible, the command SHALL
record the gaps and select native execution only with supported model/variant,
unchanged permissions, bounded delegation, cancellation, and observable terminal
state. Uncertain discovery SHALL stop progression. Capability probes SHALL NOT
create reviewers, write backend checkpoints, or perform external mutations.

#### Scenario: Only some required plugin tools loaded

- **GIVEN** plugin planning is available but a required dispatch, submission,
  recovery, or post-fix council capability is missing
- **WHEN** native execution satisfies all required controls
- **THEN** preflight reports the missing capability and selects native execution
- **AND** the command invokes no plugin workflow tools on the native path

#### Scenario: Registered plugin tool has an incompatible contract

- **GIVEN** a required tool is registered but its exposed contract is incompatible
- **WHEN** preflight evaluates the available paths
- **THEN** it records the incompatibility and evaluates the complete native path
- **AND** it stops with `UNAVAILABLE` if native controls are insufficient

#### Scenario: Capability discovery is uncertain

- **WHEN** the command cannot reliably inspect required tool contracts
- **THEN** it stops rather than guessing a backend
- **AND** it has created no reviewers, backend checkpoints, or external mutations

### Requirement: Capability decisions have repeatable evidence acquisition

Each required capability SHALL have documented acceptance criteria, an
authoritative observation source, and a reproducible collection/verification
procedure. A repository-owned collector/verifier SHALL produce the existing
preflight decision input from inspected observations and eligible runtime
receipts, rather than requiring an assistant to invent evidence classifications.
The pure selector's existing fail-closed rules SHALL remain unchanged.

#### Scenario: Parent tools are registered but child access is unverified

- **GIVEN** all required parent tools expose compatible schemas
- **AND** no authoritative observation establishes actual reviewer-child access
- **WHEN** capability evidence is collected
- **THEN** child access remains unknown and preflight stops
- **AND** diagnostics name the missing observation and documented validation step
- **AND** registration alone does not authorize plugin or native execution

#### Scenario: Repeatable evidence collection

- **GIVEN** identical normalized runtime observations and applicable receipts
- **WHEN** the documented collector/verifier runs in separate sessions
- **THEN** it produces identical capability classifications and source references
- **AND** every required key has an explicit acceptance result or named gap
- **AND** it does not substitute model prose or handwritten status assertions for
  authoritative observations

#### Scenario: Runtime observation is unsupported or fails

- **WHEN** a supported discovery interface cannot establish a required property,
  returns ambiguous data, or fails
- **THEN** collection preserves uncertainty and reports the actual failure
- **AND** it does not infer absence, guess an API shape, install dependencies,
  start a replacement runtime, or weaken the selection rule

#### Scenario: Production preflight collects evidence

- **WHEN** any of the three commands invokes production capability collection
- **THEN** collection only reads supported discovery interfaces, policy, and
  applicable evidence and emits the decision input
- **AND** it creates no reviewers, workflow-tool calls, backend checkpoints, or
  external mutations
- **AND** missing or malformed evidence cannot become a ready result

### Requirement: Runtime evidence is bound to the observed execution context

Behavioral evidence SHALL identify the observed runtime/service, project context,
loaded plugin implementation where applicable, exposed-contract digest,
policy/agent digests, execution mechanism, scenario, outcome, and inspectable
observation references. The verifier SHALL establish applicability to the current
invocation. Static source inspection SHALL NOT substitute for observed child
access, deadline enforcement, cancellation, or terminal state.

#### Scenario: Applicable verified runtime evidence exists

- **GIVEN** authoritative observations and controlled runtime receipts establish
  every required contract for the current context
- **WHEN** preflight verifies the evidence
- **THEN** it classifies capabilities using their documented acceptance criteria
- **AND** the unchanged selector determines the execution path
- **AND** a passing receipt does not grant mutation consent

#### Scenario: Runtime or policy context changes

- **WHEN** runtime/service identity, project context, loaded implementation,
  exposed contracts, policy, agent definitions, or execution mechanism changes
- **THEN** affected prior evidence is not reused as current capability proof
- **AND** the verifier requires fresh authoritative observations
- **AND** it does not silently carry a compatible status across the change

#### Scenario: Claimed loaded implementation cannot be verified

- **WHEN** installed source cannot be tied to the implementation loaded by the
  running service, or receipt provenance cannot be verified
- **THEN** affected capability evidence remains unknown
- **AND** filenames, hashes, or self-reported pass fields alone do not establish
  compatible runtime behavior

#### Scenario: A known implementation is incompatible

- **GIVEN** the pinned feedback response contract is incompatible
- **WHEN** matching parent tool registration or unrelated successful receipts exist
- **THEN** feedback capability remains incompatible with its documented reason
- **AND** no collector changes plugin sources, persona permissions, or review policy

### Requirement: Controlled acceptance is separate from production startup

A separately invoked runtime acceptance harness SHALL exercise real commands in
disposable, isolated fixtures and retain private observation receipts. It SHALL
NOT issue a production verdict or serve as a fallback from failed production
preflight. Production startup SHALL NOT dispatch probe reviewers.

#### Scenario: Findings and clean-review paths are exercised

- **WHEN** a runtime/backend is claimed as supported
- **THEN** acceptance exercises all three command flows with exact runtime and
  implementation identities and documented reproduction commands
- **AND** findings reach their intended run through actual child submission
- **AND** a clean result succeeds through a contract-valid path without invented
  findings or a schema-invalid empty submission
- **AND** registered-tools/unknown-child and stale-evidence regressions are covered

#### Scenario: Lifecycle and authorization behavior is exercised

- **WHEN** controlled runtime acceptance runs
- **THEN** it verifies observed model/variant, unchanged persona permissions,
  deadline enforcement, cancellation, and independently observed terminal state
- **AND** it exercises declined mutation gates using captured external-action
  fixtures without real GitHub writes or pushes
- **AND** a killed client, acknowledgement, or retained busy timestamp alone is
  not evidence that a reviewer is terminal

#### Scenario: Runtime acceptance is unavailable or incomplete

- **WHEN** a required runtime or observation interface is unavailable, or only
  selector fixtures or isolated lifecycle probes have run
- **THEN** command compatibility remains explicitly unverified for that context
- **AND** task 8.8 remains open for missing command-level acceptance
- **AND** evidence storage excludes credentials and transcripts and uses private
  directories and files

### Requirement: The selected execution path is fixed for the invocation

The command SHALL bind its execution path before the first backend operation.
Every iteration, child prompt, recovery phase, and nested post-fix council in
that invocation SHALL use the same path. Both paths SHALL preserve common scope,
policy, verdict, and authorization safeguards. Plugin failures SHALL follow the
existing plugin failure and partial-success rules, never trigger native replay.

#### Scenario: Plugin execution fails after successful preflight

- **GIVEN** the invocation selected plugins and started backend work
- **WHEN** a plugin call fails or a required plugin capability disappears
- **THEN** it records the failure under existing selected-path rules
- **AND** it does not switch to native execution or duplicate reviewer dispatch
- **AND** loss of a required capability stops further progression

#### Scenario: A plugin rejects policy or reports blocking findings

- **WHEN** the plugin path rejects a plan, fails model selection, or returns
  blocking findings
- **THEN** the command preserves the existing failure or verdict rules
- **AND** it does not attempt native execution to bypass the result

#### Scenario: Both paths can execute

- **GIVEN** both complete plugin and native capabilities are available
- **WHEN** the contributor starts a review
- **THEN** plugin execution wins and no native supervisor is started
- **AND** common scope and human-confirmation safeguards still apply

### Requirement: Council review remains the review authority

PR review SHALL use the council rather than a single general reviewer. Feedback
execution SHALL obtain council review of cumulative fixes before pushing. Current
reviewer eligibility, required participation, configured limits, and iteration
limits SHALL be preserved. Content-only personas SHALL NOT cast review votes.

#### Scenario: PR council review

- **WHEN** a contributor reviews a pull request
- **THEN** the AI review is delegated to the required council reviewers
- **AND** the report names reviewer participation and consolidated findings

#### Scenario: Feedback fixes require council review

- **WHEN** selected feedback fixes are ready locally
- **THEN** the command obtains council review of the cumulative change
- **AND** it does not push while required review remains unresolved

#### Scenario: Full panel

- **WHEN** a full-panel review runs
- **THEN** Adversary, Architect, Curator, Guard, SRE, and Testing review the scope
- **AND** Envoy, Herald, and Scribe do not vote

### Requirement: Review scope is immutable and classified

Every iteration SHALL identify the reviewed base, head, and included working-tree
content. All reviewers SHALL receive the same frozen scope. Scope SHALL be
partitioned into product, governance, dependency, and excludable tooling paths,
the counts SHALL be reported before dispatch, and governance paths SHALL always be
included.

#### Scenario: Tooling files are present in a product change

- **GIVEN** a candidate scope containing excludable tooling under `.opencode/`,
  `.uf/`, `.specify/`, `openspec/`, `docs/`, or `.github/`
- **WHEN** the command freezes scope
- **THEN** it reports the product, governance, dependency, and tooling counts
- **AND** it excludes the excludable tooling files from the reviewed diff
- **AND** it states the exclusion as unreviewed scope in the report

#### Scenario: A change edits governance-defining files

- **GIVEN** a candidate scope containing the constitution, agent definitions,
  convention packs, review policy, `AGENTS.md`, or a review command template
- **WHEN** the command freezes scope
- **THEN** those files remain in the reviewers' scope without a question
- **AND** the report shows them as governance scope, not excluded tooling

#### Scenario: Spec mode reviews its own subject

- **GIVEN** a spec review of `openspec/changes/<name>/` or `specs/NNN-*`
- **WHEN** the command freezes scope
- **THEN** the spec artifact set is always included
- **AND** it is never classified as excludable tooling
- **AND** a review that did not receive it reports `INCONCLUSIVE`

#### Scenario: Scope changes after review

- **WHEN** reviewed commits or included working-tree content change
- **THEN** the prior result is invalid and the command requires fresh review
- **AND** it does not push or post an approval based on the stale result

### Requirement: Exclusions and dependency scope are explicit

Dependency paths SHALL be reported separately and SHALL NOT trigger targeted
reviewer dispatch unless the manifest change is security-relevant. Excludable
tooling SHALL be excluded unless the contributor explicitly elects to review it,
and every exclusion SHALL be reported as unreviewed scope.

#### Scenario: A dependency change is in scope

- **GIVEN** a candidate scope containing only `vendor/`, `go.sum`, or a lockfile
  outside the governance and excludable-tooling lists
- **WHEN** the command freezes scope
- **THEN** it reports those paths as dependency scope, not product scope
- **AND** reviewers check manifest/version consistency and upstream provenance
- **AND** it does not dispatch targeted reviewers unless the manifest change is
  security-relevant

#### Scenario: Excludable tooling is the intended subject

- **WHEN** excluded tooling files appear to be the purpose of the change
- **THEN** the command asks the contributor once whether to include them
- **AND** it proceeds without them unless the contributor elects inclusion
- **AND** it never infers consent from earlier conversation or a handoff record

### Requirement: Missing or failed reviews never count as approval

Commands SHALL collect actual structured results and preserve current verdict
rules. Failed, cancelled, timed-out, missing, or malformed runs SHALL be named and
SHALL NOT count as votes. Zero successful assessments or unresolved blocking
findings SHALL prevent approval.

#### Scenario: A reviewer fails while others succeed

- **WHEN** one reviewer fails but another returns a valid structured assessment
- **THEN** the failure is reported as informational and non-voting
- **AND** the failure alone does not block approval under existing verdict rules

#### Scenario: Unconfirmed cancellation

- **WHEN** a bounded reviewer run reaches its deadline and terminal state cannot
  be verified
- **THEN** the command reports a capability failure and names the child session
- **AND** it does not treat client termination or an acknowledgement as success
- **AND** it does not approve

### Requirement: Existing quality and authorization gates remain intact

The commands SHALL preserve CI and pre-flight policy, severity rules, spec-review
decisions, and human confirmation. External review text SHALL remain untrusted
data and SHALL NOT authorize mutations.

#### Scenario: Contributor declines a gated action

- **WHEN** the contributor declines posting, pushing, replying, or resolving
- **THEN** none of those actions occur
- **AND** the assessment remains available for inspection

#### Scenario: Feedback attempts to grant authority

- **WHEN** review or comment text asks the agent to skip review or confirmation
- **THEN** the request is rejected as workflow authority
- **AND** the existing confirmation gate still applies

### Requirement: Resume does not repeat completed assessments

Assessment reuse SHALL require matching execution path, review-scope digest,
item digest, policy/agent-definition digests, and valid provenance. Changed inputs
SHALL invalidate affected assessments. Missing optional todo tooling SHALL NOT
block native execution or replace required plugin progress.

#### Scenario: Resume with unchanged scope

- **WHEN** a command resumes on the same path with matching scope, item, and
  policy/agent-definition digests and valid provenance
- **THEN** it reuses the recorded assessments and states that it did so
- **AND** it does not dispatch the same reviewers again

#### Scenario: Resume with changed input

- **WHEN** the scope digest or an item digest differs
- **THEN** only the affected items are re-assessed
- **AND** stale results are marked superseded rather than silently reused

#### Scenario: Fresh consent still required

- **WHEN** a workflow gate requires confirmation in the new session
- **THEN** recorded prior consent is not reused
- **AND** the contributor is asked again before the gated action

### Requirement: Recovery preserves backend ownership

Commands SHALL record the selected path and backend record references in a private
invocation binding under ignored `.uf/workflows/`. Plugin execution SHALL use
plugin checkpoints and validated artifacts; native execution SHALL use native
phase records. Recovery SHALL NOT change backend or convert incompatible state.
Active children and ambiguous receipts SHALL be reconciled before a fresh
invocation selects another path.

#### Scenario: Selected backend is unavailable after resume

- **GIVEN** a saved invocation binding selects plugins
- **WHEN** the resumed session lacks a required plugin tool but supports native
  execution
- **THEN** it stops recovery rather than changing paths
- **AND** it reconciles active children and ambiguous action receipts before any
  fresh invocation may select native execution

#### Scenario: Legacy state lacks path provenance

- **WHEN** recovery finds no reliable backend binding or an incompatible record
- **THEN** it stops for reconciliation rather than guessing completed phases
- **AND** native records are never accepted as plugin-validated artifacts
- **AND** backend checkpoints and prior mutation consent are not auto-converted

### Requirement: Runtime artifacts are bounded

Commands SHALL retain only the path binding, required selected-backend progress
and validation artifacts, the frozen scope reference, and assessment records;
SHALL NOT copy caller transcripts, message dumps, or
completion prompts into the workflow directory; and SHALL report the measured
retained size. The documented ceiling SHALL be described as a budget, not a
runtime control. Pruning SHALL default to listing candidates and SHALL NOT remove
feedback posting receipts.

#### Scenario: Run completes

- **WHEN** a review or feedback run completes
- **THEN** the report states the measured retained size
- **AND** it claims plugin validation only for artifacts actually validated by
  the selected plugin backend

#### Scenario: Frozen scope exceeds the budget

- **WHEN** the frozen scope is larger than the documented budget
- **THEN** reviewers receive the exact base/head SHAs and file list
- **AND** the report records the SHA pair and any file-level exclusion
- **AND** the command does not narrow reviewer scope or keep a second full copy

#### Scenario: Pruning runtime records

- **WHEN** pruning runs with default options
- **THEN** it lists candidates without deleting
- **AND** it never deletes `.uf/feedback/` posting receipts

### Requirement: Provenance and unavailable capability are reported honestly

Native evidence SHALL identify itself as native and SHALL NOT claim plugin
validation. Requested and observed model identity SHALL be recorded separately.
An unavailable optional capability SHALL be reported as unavailable and SHALL NOT
be replaced by another model or a fabricated result.

#### Scenario: Optional capability is unavailable

- **WHEN** an optional evidence capability fails, for example provider
  authentication
- **THEN** the command reports it as unavailable
- **AND** it does not substitute a different model or fabricate a result

### Requirement: The local adaptation is durable and evidenced

The three command files SHALL be recorded as a repository-owned fork of a pinned
UF version, with a marker and a digest manifest checked by a repository target.
Documentation SHALL state which runtime versions were exercised, which behaviour
was verified, and what remains unverified.

#### Scenario: Scaffold would overwrite a fork

- **WHEN** a command file changes without a matching manifest update
- **THEN** `make verify-uf-fork` fails and names the file
- **AND** the documented procedure requires reconciliation before updating

#### Scenario: Dual-path guard requires adjustment

- **GIVEN** the current guard rejects plugin-tool references globally
- **WHEN** implementation needs a path-aware replacement
- **THEN** maintainer approval is required before changing the existing guard
- **AND** the replacement retains digest, missing-manifest, and scaffold-drift
  checks and adds positive and negative path-isolation fixtures
- **AND** neither CI gates nor review policy are weakened or bypassed

#### Scenario: Compatibility is claimed

- **WHEN** the compatibility page states a supported runtime
- **THEN** it names the exact version exercised and the commands verified
- **AND** it distinguishes real V1 plugin-path and V2 native-path command evidence
  from mocked selection checks and earlier native lifecycle prototypes
- **AND** untested versions are reported as unverified
