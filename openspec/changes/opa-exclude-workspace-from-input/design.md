# Design

## Context

See [proposal.md](proposal.md) and [issue #231](https://github.com/complytime/complytime-providers/issues/231).
`LocalPathLoader.Load` returns a cleaned input path; the server passes it to
`scan.EvalPolicyWithNamespaces`. `constructConftestTestCommandWithNamespaces`
does not enforce an exclusion. `ServerOptions.WorkspaceDir` defaults to the
relative `provider.WorkspaceDir` (`.complytime`) and determines where the provider
writes its own artifacts. Separately, complyctl injects its resolved workspace base
into every target's `workspace` variable, so the two can disagree when the host
runs with a non-default workspace.

Conftest v0.68.2 walks directory inputs recursively. Its `--ignore` regular
expression is tested against file paths, not directory entries: ignored directories
are still traversed and may still cause filesystem errors. Its walker can follow
directory symlinks and resolves file symlinks after the ignore check. Consequently,
an expression matching the lexical workspace path alone does not reliably exclude
all alias paths. This is a source-based assessment; reproduce with the actual tool
before implementation is considered verified.

## Goals / Non-Goals

**Goals:**

- Keep locally selected target data separate from the configured workspace without
  interfering with intentional remote scans.
- Preserve existing ordinary local-file and external-directory behavior, subject to
  the alias and empty-input checks, and fail visibly when safe exclusion cannot be
  established.
- Test the actual conftest behavior, not only the command arguments.

**Non-Goals:**

- A new user-configurable ignore option, workspace relocation, or cleanup.
- Changes to evidence formats or OPA's working-clone lifecycle.
- Full protection against adversarial concurrent filesystem mutation between
  validation and conftest traversal; the workspace is not a sandbox.

## Decisions

### D1: Apply target-specific workspace boundaries

The host-injected `workspace` target variable carries the workspace *base*
directory, not the artifact directory. Resolve the boundary as that base joined with
complyctl's workspace directory name (`provider.WorkspaceDir`), so a target such as
`input_path: .` stays eligible while its `.complytime` subtree is excluded. Fall
back to the configured workspace directory when no variable is injected, and treat
the provider's own artifact directory as an additional boundary when it differs from
the resolved one. Resolve each boundary to an absolute, symlink-resolved path before
comparison. This change does not relocate provider artifacts; aligning that
directory with the host workspace is separate work.

Use that resolved boundary as the exclusion boundary for local input. An explicit
local input equal to or inside that workspace is rejected with an actionable error,
including regular files and symlink-resolved paths. An enclosing local directory is
eligible after exclusion is established. For remote targets, the deliberately
selected clone is eligible even though it lives in the workspace. Resolve the clone
root and selected scan path, and inspect symlinks that conftest would encounter.
Reject a target before conftest evaluation if the scan path or a traversed symlink
resolves into workspace content outside the clone, resolves to a file whose identity
matches workspace content outside the clone, or if relevant resolution fails. The
clone subtree is deliberately selected, so omit paths under the clone while building
the workspace path and identity sets; otherwise every clone file would match itself.
Filter by path during collection rather than subtracting identities afterwards, so
an identity still reachable at a workspace path outside the clone remains detected.
Bound traversal with visited-directory tracking so symlink cycles terminate and
reject rather than loop. Non-cyclic internal clone symlinks remain eligible. This is not a general sandbox for paths
outside the workspace. When selection fails, use existing target-level operational
error routing and do not synthesize passing assessments for that target.

For local and remote directory targets, an empty effective input set or conftest
output that does not establish that an eligible input was evaluated is also an
operational error, not a successful scan. This covers empty directories,
workspace-only inputs, empty clones or scan paths, and inputs fully removed by
effective ignore configuration. Inspect conftest's
actual structured output and error behavior for v0.68.2 in real-tool tests; do not
infer evaluation from a successful process exit, a zero finding count, or an empty
JSON array. Apply this rule per target before synthetic assessment generation, while
preserving results for other targets. A directory with confirmed evaluated input
still uses the existing target-scoped mapping and synthetic-pass behavior.

For a configured remote target selecting multiple branches, an input-safety
preflight rejection or lack of confirmed evaluated directory input in any branch
invalidates assessments for the entire configured target. Discard results already
accumulated for its other branches and do not publish successful results from later
branches. Preserve the rejecting branch's actionable operational error and accumulated
result-write errors, and continue processing other configured targets. Cancellation
already follows this target-level rule. Distinguish the new input-selection failures
using structured error classification, not error-message matching; do not broaden
the change to unrelated legacy clone or policy-loading error behavior. This does not
change clone cleanup or remove previously written per-branch artifacts.

Regression tests MUST exercise rejection before and after a successful branch,
workspace symlink and hard-link aliases, empty and fully ignored directory branches,
missing evaluated-input evidence, cancellation, and a separate successful configured
target. An all-valid multi-branch control MUST retain normal assessments.

**Alternatives:** Filtering only retained OPA results misses other providers,
Evaluation Logs, generated config, and clones. Rejecting every enclosing directory
would prevent common `input_path: .` usage.

### D2: Prefer conftest filtering, but verify aliases

For ordinary directory overlap, derive an anchored, regex-escaped `--ignore`
expression from the actual input and workspace paths, not a hardcoded component.
Match the exact spellings the conftest walker sees, including relative and absolute
forms and path-component boundaries.

Conftest's `--ignore` is a single string, and a flag overrides both `CONFTEST_IGNORE`
and any `conftest.*` configuration file. Passing the workspace expression alone would
silently discard a user's ignore rule. Reading that rule faithfully would require
reproducing conftest's configuration loader, so the provider instead determines
whether a user rule can exist and fails closed when it cannot tell:

- A nonempty `CONFTEST_IGNORE` is composed with the workspace expression into one
  grouped `--ignore` argument, whether or not a config file exists: the environment
  value already wins over the file for this setting. The variable itself stays
  unchanged, because dotenv configuration can interpolate it into other settings.
- When that variable is empty or unset and no config file applies, the workspace
  expression is used alone.
- Otherwise the overlapping target is rejected with an actionable operational error.
  It names the config file, or, when detection itself failed, the searched location
  and the reason. Both state the remedies: select a target that does not overlap a
  workspace boundary, or set `CONFTEST_IGNORE` to the effective rule — including the
  file's existing `ignore` value, or any valid expression that matches nothing when
  the file defines none. Other settings in the file continue to apply.

Detection uses the working directory and environment of the conftest process the
provider will run, resolving a relative `CONFTEST_CONFIG_FILE` against that
directory, and considers the configuration filenames covered by real-tool contract
tests. Those tests compare discovery against the installed conftest executable rather
than using its version string as a proxy for behavior.
Enumeration is not best-effort: when presence cannot be established and no
environment rule is set, reject rather than assume none applies. Detection is
irrelevant once a nonempty `CONFTEST_IGNORE` is composed, since that value already
overrides the file.

This trades a narrow configuration case for a guarantee that user ignore rules are
never silently dropped. Rejection applies only to local targets that overlap a
workspace boundary; non-overlapping local targets and remote clones never need the
workspace expression and are unaffected.

Resolve actual workspace and local input paths for overlap decisions. Inspect
symlinks that conftest would encounter, including aliases to workspace files and
directories and a symlinked workspace root. Account for conftest's resolution order:
it checks a symlink path against the regex before it resolves the target, so a
file symlink pointing into the workspace cannot be secured by an ordinary workspace
prefix alone. Exclude such aliases only when the chosen mechanism demonstrably
works; otherwise return an operational error for the target before evaluating it.
If alias resolution or relevant traversal fails, fail closed rather than publish
potentially contaminated results. Keep path operations component-aware, not string
prefix checks.

Hard links have independent pathnames but share file identity. Compare relevant
regular target files, including explicit local files and files reached through
symlinks in local or remote targets, with workspace regular files outside any
deliberately selected clone, using filesystem identity rather than path spelling or
content hashes. Exclude a
matching alias only if conftest cannot evaluate it; otherwise reject the target. If
relevant identities or workspace traversal cannot be established reliably, reject
the target. Cover hard links across distinct input directories, and retain ordinary
external files that have no workspace identity match. These checks share the existing
non-guarantee against concurrent filesystem mutation.

Use the existing runner boundary and keep provider-specific policy evaluation in
the OPA packages. Avoid a second implementation of conftest's file-type selection.
A preflight for paths and symlinks may be necessary, but it is not an alternate
parser or a claim of race-free isolation.

Preflight must observe scan cancellation during traversal and return an operational
error without assessments when cancelled. Exercise a representative large tree in a
test or benchmark, recording fixture size and elapsed time without introducing a
flaky wall-clock CI threshold. Measure workspace and target sizes separately and
avoid repeated pairwise identity comparisons: aim for work proportional to the
workspace and target entries inspected per target, excluding filesystem latency.
Avoid retaining file contents in memory while comparing identities. As a review
criterion rather than a CI gate, preflight over the representative fixture should
stay well under the scan's own runtime — on the order of seconds, not minutes — with
memory proportional to entry metadata. Investigate results that exceed it before
merging.

**Alternatives:** Moving evidence outside the workspace changes the broader storage
model. Enumerating and parsing target files in the provider duplicates conftest's
supported formats, and real-tool probes confirmed conftest does not apply its ignore
rule to explicitly listed files, so passing a file list would evaluate files the user
excluded. Vendoring conftest's configuration library would preserve every case but
adds a large dependency for a narrow need. A lexical-only ignore pattern misses
symlink aliases. An upstream conftest option that composes with its own effective
ignore would remove the rejection case; propose it separately rather than blocking
this fix.

### D3: Be explicit about traversal and diagnostics

When exclusion is applied, emit a concise informational diagnostic identifying the
excluded workspace without logging target variables. `--ignore` does not prune
directories. An unreadable directory or broken symlink inside a filtered tree can
therefore still cause a traversal failure; report it as an operational error,
never as a compliance failure or passing result. Do not claim exclusion makes the
workspace invisible to the filesystem walker.

Document the explicit-workspace-target behavior and the workaround: copy intended
input to a location outside the workspace. Explain that enclosing directory scans
also stop evaluating workspace files, including user-owned files, and document
empty-input errors, hard-link handling, traversal, and concurrent-mutation limits.

### D4: Verify against a real conftest

Unit tests cover path relationships, regex escaping, arguments, errors, and target
routing using existing test seams. A separate real-conftest regression creates a
temporary input tree and policies, then verifies actual filenames and assessments
for valid and malformed workspace fixtures, repeated scans, symlinks, and controls.
It must assert that no workspace fixture contributes to results, and include a
positive non-workspace finding to avoid a vacuous pass. Where a case cannot be
handled safely, assert an operational error with no assessments.

Conftest compatibility is behavioral, not an exact-version runtime policy. Remove
the version allowlist, version-detection prerequisite, and source-build module
metadata fallback. Missing, unusual, or `dev` version output MUST NOT by itself
prevent local or remote evaluation. Keep executable-presence checks and fail closed
on actual input-safety, execution, or output-validation failures. These checks are
not executable authentication or a sandbox against filesystem mutation.

The real-tool suite MUST run against any installed conftest executable, not skip
because its version differs from v0.68.2 or cannot be determined. Record version
output when available for diagnostics; failure to obtain that output MUST NOT skip
behavior tests. A discovered executable that cannot perform the tested operations
MUST fail the suite. Cover ignore precedence across flags, environment, and config
files; configuration discovery; explicit-file ignore bypass; symlink traversal and
resolution order; structured evaluation evidence; and empty-input behavior. Assert
selected filenames, findings, success counts, and operational-error outcomes rather
than treating an exit code or version string alone as proof of compatibility.

Route every operational error added by this change through the existing Scan
response's operational errors, not provider logs alone; test that a normal
`complyctl scan` displays them and exits nonzero, including a scan mixing rejected
and successful targets. Update the CLI fixture so a different or placeholder version
does not prevent otherwise valid evaluation.

The local CI workflow does not install conftest. The real-tool test may skip when
the executable is absent, but the implementation must record a separate successful
run with v0.68.2 and at least one different available release; a skipped test does
not count. Record each actual version and command without inventing a supported
version range. Track a separate cross-repository follow-up with the `complyctl`
harness maintainers for recurring non-skipping coverage of changing releases,
without changing this repository's CI gates. Record its issue link before
the implementation PR merges. Until that follow-up is in place, recurring automated
compatibility coverage is a known gap; do not claim existing file-only cross-repo
tests exercise this behavior. Tests only detect changes in versions actually
exercised; removing the runtime pin does not guarantee compatibility with every
future release. Document that limitation and the tested versions.

## Risks / Trade-offs

- **Symlink and path ambiguity** → Fail affected local targets when safe exclusion
  cannot be established; cover file and directory aliases and custom workspaces.
- **Concurrent filesystem changes** → Preflight is not a sandbox; document the
  limitation and do not claim protection against adversarial races.
- **Traversal despite ignore** → Unreadable filtered directories may still fail a
  scan; document and test operational-error handling.
- **Intentional workspace scan breaks** → Reject explicitly and document copying
  files outside the workspace as the workaround.
- **Filtered files can include user-owned content** → Each resolved workspace is
  reserved for complyctl use. Files outside every resolved workspace boundary remain
  eligible, except deliberately selected remote clones, which stay evaluable.
- **Tool behavior can change** → Run executable behavior regressions against
  v0.68.2 and another available release; do not reject versions by string alone.
  Recurring automated compatibility coverage remains a gap until separately
  addressed. Tests do not establish compatibility with untested future behavior.
- **Partial remote target results** → Discard all branch assessments for a configured
  target rejected by the new input-selection checks, without suppressing successful
  configured targets or deleting previously written branch artifacts.

## Migration Plan

1. Review and approve this proposal, then commit and push the specification artifacts.
2. Implement selection and validation without changing the RPC interface; update the
   OPA man-page source, generated page, README as appropriate, and release notes.
   Ensure the generated GitHub release notes disclose both explicit-target errors
   and the changed results of enclosing-directory scans, and explain the copy-out
   workaround. Verify the actual release-note mechanism rather than assuming a
   `CHANGELOG.md` entry appears in GoReleaser's GitHub-generated changelog.
3. Run unit tests, the real-conftest regression, strict OpenSpec validation, and
   local CI checks. Track the cross-repository recurring-regression follow-up.
4. Merge this change before `retain-scan-evidence`, or incorporate equivalent
   coverage there. Rolling back restores the previous unfiltered behavior.
