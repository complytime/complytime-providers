# Design

## Context

See `proposal.md` for motivation and the capability delta for FR-001
through FR-007. The current workflow independently resolves a stable
complyctl baseline, verifies its checkout, replaces the dependency in
an isolated provider worktree, builds/tests those providers, and builds
the released complyctl and registry. It then runs the historical
checkout's `make test-cross-repo` target.

The provider Make target already accepts `COMPLYCTL_DIR` as the harness
checkout and uses a bounded invocation of its script. The companion
complyctl change adds `COMPLYCTL_BIN_DIR` to select both host binaries
without moving fixture or assertion ownership.

The completed but unpromoted change
`validate-complyctl-release-compatibility` chose release-owned
assertions. Its design's released-harness decision and the diagnostic
test now conflict with the approved current-harness direction. This
change records that conflict rather than silently revising history.

## Goals / Non-Goals

**Goals:**

- Preserve release provenance and dependency/build checks independently
  of where assertions evolve.
- Use one canonical, reviewable implementation of evidence assertions.
- Make wrong-host execution detectable without live credentials.

**Non-Goals:**

- Copy or maintain an Ampel evidence checker in this repository.
- Alter provider code, evidence aliases, committed dependency versions,
  release compatibility-line selection, or the existing main job.
- Rebuild released source using current implementation files.

## Decisions

### Preserve three distinct revision identities

Record the provider revision, selected released host tag/commit, and
canonical harness commit. The host resolver remains unchanged. Add a
single workflow-owned full commit pin for the canonical harness after
the companion complyctl PR merges; verify the checked-out HEAD equals
that pin and report it with the released host identity.

The pin must reference a real reviewed commit containing the required
interface and assertions. Do not invent a SHA during planning or point
at an unmerged branch. Updating the pin later requires a reviewed PR
and the same execution regressions; do not resolve it from mutable main
at runtime. The separate main-consumer job continues tracking main.

### Reuse the canonical script without modifying released source

Use separate checkouts for the released source and the canonical
harness. Keep building the released binaries with its existing build
target. Build and test providers in the isolated worktree with its
dependency replaced by that exact released source as before.

Invoke the provider repository's existing `test-cross-repo` target with
`COMPLYCTL_DIR` set to the canonical harness checkout and absolute
`COMPLYCTL_BIN_DIR` set to the released checkout's `bin/`. Export the
existing provider binary directory and least-privilege token only to
integration as required. Do not invoke the canonical complyctl build
target during this job.

```text
Provider revision --> Isolated replacement --> Provider build + tests
                            ^                         |
                            |                         v
Released host commit --> Released host binaries <-- Canonical harness
                                                   + own testdata
                                                   + evidence checks
                                                   + immutable pin
```

The canonical harness's mock-registry executable also comes from the
released binary directory. Fixture paths belong to the harness. The
companion design verified no source differences in those fixture and
registry areas between local v1.2.0 and main on 2026-11-13; execution
must still validate the contract. If a future pin requires newer
registry features, report incompatibility rather than mix revisions.

Reject copying the new script into the historical tree, patching old
glob lines, or supplying a main binary as an apparent release build.
Those approaches make provenance ambiguous and weaken reproducibility.

### Keep preparation and diagnostic responsibilities separate

Add focused harness preparation checks for verified revision, required
script/fixtures, and supported binary-selection input. Keep released
checkout verification, build prerequisites, and binary validation
separate. The historical harness may still be checked for repository
integrity, but its existence must not select which assertions run.

Reuse stage-specific diagnostics and add a distinct
`canonical-harness-preparation:` category. Report both commits in the
job summary. Preserve integration output and its exit status; redact
tokens through the canonical harness's existing token handling.

Do not collapse a harness checkout failure into release resolution,
or label a real provider/evidence failure as a preparation failure.

### Resolve the historical-harness protection before implementation

The existing diagnostic test requires the exact historical Make
invocation and rejects `_compatibility_harness`. These checks express
the earlier contract; choosing another directory name to evade them
would violate their intent.

The maintainer confirmed during exploration that current complyctl
owns the authoritative assertions while the selected release remains
the implementation tested. This establishes design intent, not an
automatic exemption from the repository's gatekeeping rule. A
maintainer must resolve the existing protection in a reviewed change
before an implementation agent can replace the historical invocation.
Stop if that resolution has not occurred. Do not remove the tests,
change CI flags, or alter quality/governance policy to obtain a pass.

Preserve their useful guarantees through stronger execution coverage:
the released checkout remains exact, providers use its dependency,
the released host and registry actually execute, and any integration
failure remains a job failure. This planning change does not edit
those existing tests, the old spec artifacts, or any workflow.

### Prove wiring by executing it, not matching YAML alone

Add a focused helper for the released integration invocation if needed
so offline shell tests exercise the same command path as the workflow.
Use temporary checkouts and distinguishable host, registry, and harness
executables that append provenance to a test-owned trace. Keep the
helper limited to validation and dispatch, not build orchestration or
evidence semantics.

Positive coverage proves the selected host directory, canonical
fixture root, and isolated provider binary directory reach the command.
Negative coverage includes missing host/registry, missing harness,
revision mismatch, unsupported host selection, failed provider checks,
and nonzero integration. Assert that default or historical binaries
never execute as a fallback. Inspect workflow text only to complement
these behavior tests and verify step ordering and permissions.

## Risks / Trade-offs

- [Existing protection blocks wiring] -> Leave the implementation
  task blocked pending maintainer handling; never rename around it.
- [Pinned assertions age] -> Refresh via reviewed immutable pins,
  recording host compatibility and execution-test results each time.
- [A new harness silently requires a newer host] -> Run the released
  host explicitly and fail on unsupported CLI or registry behavior.
- [Build uses wrong module checkout] -> Preserve isolated dependency
  replacement and provider build/test steps before integration.
- [Offline stubs overstate real coverage] -> Require credentialed
  released-host and main-host integration results separately.
- [Two unpromoted specs share one capability] -> Reconcile the earlier
  release-compatibility change before archiving either into main specs;
  never archive the old harness requirement after this supersession.

## Migration Plan

1. Review and commit/push this repository-local planning set separately
   from the provider retention implementation.
2. Wait for the companion complyctl harness PR to merge. Verify and
   record its full commit as the initial canonical harness pin.
3. Obtain maintainer resolution of the historical-harness protection;
   until then, implementation of affected wiring is blocked.
4. Implement harness checkout/preparation and explicit host dispatch,
   preserving the resolver and provider dependency/build checks.
5. Run all baseline shell suites, added execution regressions, workflow
   lint, provider build/tests/lint, and applicable CI-derived checks.
6. Run real retained-provider integration with the selected released
   host and current host. Record both revisions, tools, and actual job
   results; missing credentials remain an incomplete verification task.
7. Merge this separate PR after the canonical harness commit exists and
   required review succeeds. No paired host release is necessary.

Rollback must not restore knowingly obsolete checks or introduce
aliases to hide failures. Revert only with a documented known-good
host/provider/harness combination; otherwise keep CI failing visibly
until a reviewed compatible combination is available.

Update integration usage documentation and assess `CHANGELOG.md`,
`AGENTS.md`, and `README.md`. Keep generated provider man pages and
provider runtime documentation outside this workflow-only change.
