# Proposal

Status: Plugin-first revision specified on 2026-11-17; implementation and
verification of this revision are pending in task group 8. Tasks 1-7 record the
earlier native-fork work, not dual-path acceptance. The maintainer has managed
upstream issue disposition (task 6.3); no additional posting is needed for this
PR. Existing references are [UF #1176][drift-issue],
[org-infra #747][evaluation-issue], and [org-infra #746][guidance-issue].
The UF #962 comment was declined and not posted. Future external actions
require preview and explicit maintainer approval. The fork guard runs locally
only, not in CI.
This revision retains the containment scope and change name: keep the three main
review commands usable when plugins are unavailable, without replacing working
plugin-backed execution.

## Why

PR #236 moved the repository to UF v0.21.0, whose four plugins target the
OpenCode V1 plugin API. On the installed OpenCode v2.2.7 every plugin fails to
load:

```text
PluginModule.LoadError: Plugin must export a default definition with an id and an
effect or setup function.
```

Consequences measured across the 29 top-level sessions since the merge
(2026-11-05 17:27 through 2026-11-16), using the local session store and the
OpenCode log:

- **Zero** calls to any UF plugin tool. All 52 reviewer child sessions were
  started by hand-written shell/API supervisors.
- 8 of 29 sessions ended by asking the contributor to start a fresh session; 7
  were resumed from a `/tmp/opencode/*-handoff.md` file. `/uf.address-feedback`
  was split across two sessions (134 min + 132 min) that assessed the same two
  feedback items twice.
- Parent sessions carry orchestration the plugins used to provide: 52-110 shell
  calls per review or feedback run, and 4-17 MB of per-run artifacts. The ignored
  `.uf/` tree holds 60 MB across eight runs.
- Because the contributor's command adaptations and a convention-pack change were
  uncommitted on the reviewed branch, a review captured 80 files and 605 KB,
  including tooling files. That produced a governance detour (issue #242, PR
  #243) and zero source edits in the first 134 minutes of the feedback run.
- Two sessions failed outright on provider authentication (`HTTP 401`), one of
  them while reporting that the optional Gaze capability was unavailable.
- The working fix is uncommitted in `.opencode/commands/` on the feature branch
  and would be overwritten by `uf init --force`. The specification that was
  supposed to make it durable sits on an unmerged branch at 3 of 15 tasks.

Upstream already tracks the root cause: UF issues #714 and #797 (OpenCode V2
migration, unreleased as of v0.21.0) and #962 (this repository's blocked
`/uf.address-feedback`). This change does not port plugins or replace UF. It
contains the local working set so it is durable, honest, and cheap until a
V2-compatible UF release exists.

The native fork also removed plugin execution instructions unconditionally.
Although its preflight describes a plugin banner, it does not provide a complete
plugin-backed command path. Contributors whose OpenCode V1 runtime loads the
plugins should retain their existing workflow. Selection must depend on usable
capabilities, not a V1/V2 version test, so a future compatible V2 plugin release
can use the same plugin-first path.

## What Changes

- Maintain a dual-path command set for `/uf.review-council`,
  `/uf.review-pr`, and `/uf.address-feedback` as a **repository-owned fork of the
  UF v0.21.0 scaffold**, with a marker, a recorded digest manifest, and a pinned
  upstream version.
- Prefer the original **plugin-backed execution** when the complete required
  tool contract for that command is available, including child submission,
  recovery, and post-fix review dependencies.
- Fall back to **native execution only during preflight**, when a required plugin
  capability is unavailable or incompatible and native controls are sufficient.
  Report missing capabilities and the chosen path once; stop if neither works.
- Bind one path to the invocation and its recovery state. Plugin execution
  failures, policy rejections, and unavailable models never authorize a switch
  to native execution or a second assessment.
- Preserve scaffold-drift detection while planning a path-aware replacement for
  the guard's blanket rejection of plugin-tool references. Changing the existing
  guard requires maintainer approval before implementation; it is not changed
  or disabled by this spec revision.
- Enforce **scope hygiene**: excludable tooling paths are excluded from review
  and feedback scope unless explicitly elected, governance and dependency paths
  are reported, and unreviewed paths are named in the report.
- Make **resume fidelity** explicit: assessments are keyed by review-scope
  digest, item digest, and execution-path provenance. A resumed run reuses valid
  unchanged assessments on the same path, not incompatible backend checkpoints.
- Bound **artifacts**: per-run retention limits, no copies of caller transcripts,
  and a cleanup target for `.uf/workflows/`.
- Add `docs/review-command-compatibility.md` recording the measured runtime
  status, the safeguard reconciliation, and the prototype evidence, and record
  the pinned version, drift guard, and upgrade trigger in `AGENTS.md`.
- Retain the historical **upstream drafts** and maintainer-managed disposition
  (see `issue-drafts.md`); further publication is not implementation work and
  requires content preview and explicit approval.

Out of scope: porting the four plugins, changing council eligibility, model
policy, limits, verdicts, severity rules, convention packs, personas, or the
constitution; repairing other UF commands; and migrating to another agent
platform. A Fullsend evaluation is a separate organization-level decision
recorded in the drafts; no part of it is implemented here.

This is one contributor story: run the three review commands on the installed
OpenCode runtime, retain working plugins when available, see which path executed,
and resume without paying twice.

## Capabilities

### New Capabilities

- `review-command-compatibility`: durable, honest, bounded execution of the three
  main review commands, preferring available UF plugins and otherwise using
  native execution, preserving council review and mutation safeguards.

### Modified Capabilities

None. Existing OpenSpec capabilities describe provider and release behavior, not
contributor review commands.

## Impact

- Primary surface: `.opencode/commands/uf.review-council.md`,
  `uf.review-pr.md`, `uf.address-feedback.md`, plus the drift-check target
  (repository Makefile), its shell tests, and one manifest under `.uf/`.
  The guard adjustment is approval-gated and must retain digest verification
  and coverage of accidental scaffold overwrites.
- Unchanged: `.opencode/agents/`, `.opencode/uf/packs/`, `.uf/review-matrix.yaml`,
  `.uf/reviewer-capabilities.yaml`, `.uf/schemas/`, plugin sources and
  registrations, and every other UF command. Other workflows still consume the
  plugin registrations, so they are not removed here.
- No new runtime packages, no V2 plugin adapter, no global OpenCode
  configuration, and no CI gate changes.
- Documentation: `docs/review-command-compatibility.md`, `AGENTS.md`, and
  `CHANGELOG.md` will need dual-path wording and separate V1-plugin/V2-native
  command evidence during implementation. This revision changes spec artifacts
  only and makes no new compatibility claim. `docs/AI_TOOLING.md` is provisioned from
  `org-infra/docs/AI_TOOLING.md`; the command-naming and Fullsend notes are
  proposed upstream instead of edited locally.
- Related upstream work: [UF #714][migration], [UF #797][umbrella], and
  [UF #962][downstream].

[migration]: https://github.com/unbound-force/unbound-force/issues/714
[umbrella]: https://github.com/unbound-force/unbound-force/issues/797
[downstream]: https://github.com/unbound-force/unbound-force/issues/962
[drift-issue]: https://github.com/unbound-force/unbound-force/issues/1176
[evaluation-issue]: https://github.com/complytime/org-infra/issues/747
[guidance-issue]: https://github.com/complytime/org-infra/issues/746
