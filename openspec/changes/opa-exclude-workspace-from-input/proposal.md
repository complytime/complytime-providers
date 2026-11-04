# Proposal

## Why

When an OPA local directory target contains the complyctl workspace, conftest can
evaluate generated artifacts, prior Evaluation Logs, and working clones as target
input. This can produce misleading findings or parsing errors; retaining more scan
evidence would increase the impact. See [issue #231](https://github.com/complytime/complytime-providers/issues/231).

## What Changes

- Exclude the configured complyctl workspace from local directory inputs, including
  other providers' artifacts and OPA working clones. Keep unrelated inputs and
  intentionally evaluated remote clones eligible for scanning.
- **BREAKING**: Reject an explicitly selected local workspace directory or file with
  an actionable error rather than silently evaluating workspace content. Document
  that users who intentionally want to evaluate such files must copy them outside
  the workspace.
- **BREAKING**: Enclosing directory scans no longer evaluate workspace files, even
  user-owned files. Local and remote directory targets with no confirmed evaluated
  input report an operational error instead of producing synthetic passing
  assessments.
- Resolve the excluded workspace from the host-injected `workspace` base directory,
  falling back to the configured directory, and treat the provider's own artifact
  directory as a boundary when the two differ.
- Prefer conftest's `--ignore` for ordinary directory overlap, but account for
  traversal, symlink, and hard-link limitations; fail safely when exclusion cannot be
  assured. Reject remote clone paths and symlinks that reach other workspace content.
- **BREAKING**: Because `--ignore` overrides conftest's own configuration, reject an
  overlapping local target whenever an applicable conftest configuration file could
  define an ignore rule, rather than silently discarding it. A nonempty
  `CONFTEST_IGNORE` is composed instead, and non-overlapping targets are unaffected.
- Reject a configured remote target as a whole when any selected branch fails
  input-safety preflight or has no confirmed evaluated directory input. Discard
  assessments from its other branches while preserving other configured targets.
- Use behavior-based conftest compatibility rather than an exact runtime version
  requirement. Run real-tool regressions against the installed executable, including
  releases other than v0.68.2 and source builds, without skipping on version strings.
- Add unit and real-conftest regressions for the selected input set, malformed
  workspace files, paths and symlinks, and unaffected local and remote targets.
- Document the exclusion and operational limits in the OPA man page and user guidance.

## Capabilities

### New Capabilities

- `opa-input-selection`: Selection of local and remote OPA inputs while excluding
  unintended workspace artifacts, with explicit handling of unsafe overlap and
  conftest compatibility.

### Modified Capabilities

- `target-scoped-generation`: Exclude directory targets without confirmed evaluated
  input from assessment generation, including assessments from other branches of a
  rejected configured target, without suppressing other configured targets.

## Impact

- OPA target loading, scan orchestration, conftest command construction, tests,
  and OPA documentation. No RPC/protobuf changes or new user-facing settings.
- This is a provider-local tactical change. Track recurring real-conftest coverage
  with the maintainers of the cross-repository harness as a separate follow-up;
  until it exists, automated compatibility coverage has a documented gap. That
  follow-up should exercise changing conftest releases, not only v0.68.2. This
  change does not modify CI or governance gates.
- The separate `retain-scan-evidence` change should land after this fix, or include
  equivalent input isolation before it adds retained files under the workspace.
- This proposal is specification-only. Commit and push the artifacts before
  implementation in accordance with repository workflow.
