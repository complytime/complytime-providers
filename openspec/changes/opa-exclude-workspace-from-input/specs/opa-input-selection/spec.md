# Spec Delta

## Purpose

Keep generated complyctl workspace data out of unintended OPA target evaluations
while preserving intentional scans of ordinary files, directories, and remote
repositories.

## ADDED Requirements

### Requirement: Exclude the workspace from local directory evaluation

The OPA provider MUST exclude files in the configured workspace from local
directory evaluation, including files from other providers and working clones.
It MUST continue to evaluate eligible non-workspace files and MUST NOT infer a
successful assessment from a workspace file.

#### Scenario: Workspace nested in the input

- **GIVEN** a local input directory contains a legitimate configuration file and
  a workspace with a policy-triggering fixture, an Evaluation Log, and a clone
- **WHEN** the provider evaluates the directory
- **THEN** the legitimate file is evaluated
- **AND** none of the workspace files contributes findings or success counts

#### Scenario: Invalid file under the workspace

- **GIVEN** an input directory contains an invalid JSON file under the workspace
  and a valid configuration outside it
- **WHEN** the provider evaluates the directory
- **THEN** the invalid workspace file is not parsed as target input
- **AND** the valid configuration remains eligible for evaluation

#### Scenario: Repeated evaluations

- **GIVEN** the intended local input files and policy are unchanged
- **WHEN** successive evaluations create additional workspace artifacts
- **THEN** those artifacts do not change the files evaluated as target input

### Requirement: Do not report a pass without evaluated directory input

For a local or remote directory target, the provider MUST treat an empty effective
input set, or an inability to establish that at least one eligible input was
evaluated, as a target-level operational error. It MUST NOT publish assessments,
including synthetic passing assessments, for that target. Eligible inputs are
determined after workspace exclusion and effective conftest ignore configuration.
Other targets retain their own results.

#### Scenario: Workspace is the only source of input files

- **GIVEN** an enclosing local directory has workspace files but no eligible files outside it
- **WHEN** the provider evaluates the directory
- **THEN** it reports an operational error explaining that no eligible input was evaluated
- **AND** it publishes no assessments for that target

#### Scenario: Empty or fully ignored local directory

- **GIVEN** an ordinary local directory is empty or all its inputs are ignored
- **WHEN** the provider evaluates the directory
- **THEN** it reports an operational error and publishes no assessments for that target

#### Scenario: Remote clone with no evaluated input

- **GIVEN** a remote clone or its selected scan path has no eligible files to evaluate
- **WHEN** the provider evaluates that target
- **THEN** it reports an operational error and publishes no assessments for that target

#### Scenario: Another target has evaluated input

- **GIVEN** one directory target has no evaluated eligible input and another has one
- **WHEN** the provider aggregates their results
- **THEN** the empty target has an operational error and no assessments
- **AND** the other target retains its normal target-scoped assessments

### Requirement: Reject remote input selection at configured-target scope

If any selected branch fails input-safety preflight, is cancelled during preflight,
or lacks confirmed evaluated directory input, the provider MUST suppress assessments
from all branches of that configured target, regardless of branch order. It MUST
retain actionable target/branch operational errors and accumulated result-write
errors. Other configured targets MUST retain their normal results.

#### Scenario: Successful branch before an unsafe branch

- **GIVEN** a configured remote target selects `main` and `second`
- **AND** `main` evaluates successfully but `second` fails an input-safety check
- **WHEN** the provider aggregates results
- **THEN** it reports the target and rejecting branch's operational error
- **AND** it publishes no assessments from either branch of that configured target

#### Scenario: Empty or unconfirmed branch input

- **GIVEN** one selected branch evaluates successfully
- **AND** another directory branch is empty, fully ignored, or lacks evaluated-input evidence
- **WHEN** the provider aggregates results in either branch order
- **THEN** it reports an operational error and no assessments for that configured target

#### Scenario: Rejected branch before a successful branch

- **GIVEN** a configured remote target selects an unsafe branch before a valid branch
- **WHEN** input-safety preflight rejects the unsafe branch
- **THEN** no later branch contributes assessments for that configured target

#### Scenario: Another configured target remains valid

- **GIVEN** one multi-branch target is rejected by an input-selection check
- **AND** a separate configured target evaluates successfully
- **WHEN** the provider aggregates results
- **THEN** only the separate target contributes assessments and mapping references

#### Scenario: Every selected branch is valid

- **GIVEN** every selected branch passes preflight and has confirmed evaluated input
- **WHEN** the provider aggregates the target's results
- **THEN** it retains normal assessments from all selected branches

### Requirement: Explicit workspace targets fail clearly

The provider MUST reject a local input that is the workspace itself or a file or
directory within it. It MUST report an actionable operational error and MUST NOT
publish assessments from that target.

#### Scenario: Workspace directory explicitly selected

- **WHEN** a local target selects the workspace or one of its subdirectories
- **THEN** the provider reports that workspace paths cannot be scanned locally
- **AND** it does not report a passing or failing assessment for that target

#### Scenario: Workspace file explicitly selected

- **WHEN** a local target selects a file within the workspace
- **THEN** the provider reports an operational error suggesting a copy outside it
- **AND** it does not evaluate the file as target input

### Requirement: Workspace boundary uses actual configured paths

The provider MUST resolve the workspace boundary from the host-injected `workspace`
base directory and complyctl's workspace directory name, and otherwise from its
configured workspace directory. It MUST also treat its own artifact directory as a
boundary when that differs. The provider MUST use path-component boundaries, and
MUST NOT exclude an enclosing directory or unrelated paths that merely share a
textual prefix.

#### Scenario: Different path spellings

- **WHEN** the same enclosing local directory is selected using a relative or absolute path
- **THEN** the workspace is excluded in both evaluations

#### Scenario: Similar directory names

- **GIVEN** a workspace named `.complytime` and a sibling directory named `.complytime-backup`
- **WHEN** their enclosing directory is evaluated
- **THEN** workspace files are excluded and eligible sibling files remain eligible

#### Scenario: Custom workspace path

- **WHEN** the provider uses a non-default workspace within a local directory target
- **THEN** it excludes that configured workspace rather than assuming `.complytime`

#### Scenario: Workspace base injected for an enclosing target

- **GIVEN** the host injects a workspace base that encloses the selected local directory
- **WHEN** the provider evaluates that directory
- **THEN** it excludes only the workspace directory beneath that base
- **AND** eligible files elsewhere in the selected directory remain evaluable

#### Scenario: Host-injected workspace differs from the provider artifact directory

- **GIVEN** the resolved host workspace differs from the provider's own artifact directory
- **WHEN** an enclosing local directory containing both is evaluated
- **THEN** the provider excludes content in both directories
- **AND** eligible files outside them remain evaluable

#### Scenario: Workspace variable absent

- **WHEN** no workspace variable is injected for a target
- **THEN** the provider uses its configured workspace directory as the boundary

### Requirement: Never silently discard a user ignore rule

Applying the workspace exclusion MUST NOT drop a user's conftest ignore rule. For a
local target overlapping a workspace boundary, the provider MUST apply exactly one
case, in order: compose a nonempty `CONFTEST_IGNORE` with the workspace expression;
use the workspace expression alone when no rule and no configuration file apply;
otherwise reject the target with an actionable operational error and no assessments.
It MUST leave the environment unchanged.

#### Scenario: Nonempty environment rule

- **GIVEN** `CONFTEST_IGNORE` is nonempty and the local target encloses a workspace
- **WHEN** the provider evaluates the target
- **THEN** both the user rule and the workspace exclusion apply
- **AND** the environment is unchanged

#### Scenario: Nonempty environment rule with a configuration file

- **GIVEN** `CONFTEST_IGNORE` is nonempty and a configuration file also applies
- **WHEN** the provider evaluates an enclosing local target
- **THEN** it composes the environment rule with the workspace exclusion rather than
  rejecting the target

#### Scenario: No environment rule and no configuration file

- **GIVEN** `CONFTEST_IGNORE` is empty or unset and no configuration file applies
- **WHEN** the provider evaluates an enclosing local target
- **THEN** it applies the workspace exclusion alone and evaluates eligible files

#### Scenario: Configuration file could define an unknown rule

- **GIVEN** `CONFTEST_IGNORE` is empty or unset and a configuration file applies
- **WHEN** a local target overlaps a workspace boundary
- **THEN** the provider rejects that target with an actionable operational error
  naming the configuration file and the available remedies
- **AND** it publishes no assessments for that target

#### Scenario: Configuration presence cannot be determined

- **GIVEN** `CONFTEST_IGNORE` is empty or unset
- **WHEN** the provider cannot establish whether a configuration file applies
- **THEN** it rejects the overlapping target rather than assuming none applies
- **AND** the error identifies the searched location and why detection failed

#### Scenario: Detection failure with a nonempty environment rule

- **GIVEN** `CONFTEST_IGNORE` is nonempty
- **WHEN** the provider cannot establish whether a configuration file applies
- **THEN** it composes that rule with the workspace exclusion, because the
  environment value already overrides any configuration file

#### Scenario: Non-overlapping targets are unaffected

- **GIVEN** a configuration file applies
- **WHEN** a local target does not overlap any workspace boundary
- **THEN** the provider evaluates it without adding a workspace exclusion

### Requirement: Detect conftest configuration in the evaluation context

The provider MUST determine whether a configuration file applies using the working
directory and environment of the conftest process it will run, resolving a relative
`CONFTEST_CONFIG_FILE` against that directory. Real-tool contract tests MUST compare
its configuration discovery with the installed conftest executable, including
supported filenames, discovery order, explicit paths, and unsupported extensions.

#### Scenario: Explicit configuration file

- **GIVEN** `CONFTEST_CONFIG_FILE` names a file, using a relative or absolute path
- **WHEN** the provider checks the evaluation context
- **THEN** it resolves that path against the conftest process working directory

#### Scenario: Unsupported filename

- **GIVEN** a `conftest.*` file whose extension conftest does not discover as configuration
- **WHEN** the provider evaluates an enclosing local target
- **THEN** it does not treat that file as an applicable configuration file

### Requirement: Handle workspace aliases without silent contamination

The provider MUST detect relevant symlink and hard-link aliases to the configured
workspace before local evaluation. It MUST exclude aliased workspace files or reject
the affected target with an actionable operational error when it cannot ensure
exclusion. It MUST NOT silently evaluate workspace content through an alias.

#### Scenario: Symlink points to workspace file or directory

- **GIVEN** a local input contains a symlink resolving into the workspace
- **WHEN** the provider evaluates the input
- **THEN** the aliased workspace content is not evaluated
- **AND** if exclusion cannot be assured the target reports an operational error

#### Scenario: Unresolvable or inaccessible overlap

- **WHEN** the provider cannot determine whether traversal would include workspace content
- **THEN** it reports an operational error rather than publishing potentially contaminated results

#### Scenario: Hard link to a workspace file

- **GIVEN** a selected local file or a file in a local directory target is hard-linked
  to a workspace file
- **WHEN** the provider evaluates the target
- **THEN** it excludes the alias or rejects the target with an actionable error
- **AND** the workspace file does not contribute findings or success counts

#### Scenario: Hard link reached through another alias

- **GIVEN** an eligible path resolves to a file that is hard-linked to workspace content
- **WHEN** the provider evaluates a local or remote target containing it
- **THEN** it excludes the alias or rejects the target with an actionable error

#### Scenario: File identity cannot be established

- **WHEN** relevant workspace or target file identities cannot be compared reliably
- **THEN** the provider reports an operational error and publishes no assessments

### Requirement: Use behavior-based conftest compatibility

The provider MUST NOT reject an otherwise eligible local or remote target solely
because conftest reports a different, unknown, or placeholder version. It MUST NOT
require successful version detection or embedded module metadata before evaluation.
It MUST retain executable-presence checks, input-safety preflight, execution-error
handling, and evaluated-input evidence validation.

#### Scenario: Different release with compatible behavior

- **GIVEN** an installed conftest release other than v0.68.2
- **WHEN** it safely evaluates eligible input and returns valid structured results
- **THEN** the target produces its normal assessments without a version rejection

#### Scenario: Source build or unavailable version metadata

- **GIVEN** conftest reports `dev`, has no readable module metadata, or cannot report a version
- **WHEN** it safely evaluates eligible input and returns valid structured results
- **THEN** evaluation proceeds without requiring version or module metadata

#### Scenario: Actual execution or result failure

- **WHEN** evaluation fails or cannot establish evaluated directory input
- **THEN** the provider reports an actionable operational error rather than a synthetic pass

### Requirement: Test the installed conftest behavior without a version filter

Real-conftest regressions MUST exercise any installed executable rather than skip
based on its version string or module metadata. They MAY skip when conftest is
absent, but MUST fail when an installed executable violates a tested contract or
cannot execute the required operations. Version output SHOULD be recorded for
diagnostics; failure to obtain it MUST NOT skip behavior tests. Verification MUST
record successful runs with v0.68.2 and at least one different available release.

#### Scenario: Another installed release

- **GIVEN** conftest is installed with a version different from v0.68.2
- **WHEN** the real-tool suite runs
- **THEN** it executes the same behavior assertions instead of skipping for that version

#### Scenario: Version reporting unavailable

- **GIVEN** an installed conftest executable whose version cannot be determined
- **WHEN** the real-tool suite runs
- **THEN** the behavior tests still execute and determine pass or failure

#### Scenario: Dependency behavior changes

- **GIVEN** the installed conftest changes ignore precedence, config discovery,
  explicit-file handling, symlink traversal, structured output, or empty-input behavior
- **WHEN** that change violates the input-isolation contract
- **THEN** a behavioral regression fails rather than passing on a version string or skipping

#### Scenario: Conftest is absent

- **WHEN** optional real-tool tests cannot find conftest
- **THEN** they report a missing-prerequisite skip, which does not count as verification

### Requirement: Surface input selection errors to callers

Every operational error added by this change, including workspace rejection, empty
input, alias failures, and conftest execution or output failures, MUST reach callers through the
existing operational-error response rather than provider logs alone. Each error MUST
identify the affected target and an actionable remedy.

#### Scenario: Normal CLI scan with a rejected target

- **WHEN** a normal `complyctl scan` receives a workspace, empty-input, or alias error
- **THEN** it displays that error without debug logging and exits nonzero

#### Scenario: Mixed target outcomes

- **GIVEN** one target is rejected and another is evaluated successfully
- **WHEN** a normal `complyctl scan` completes
- **THEN** it reports the rejected target's error, retains the successful target's
  assessments, and exits nonzero

### Requirement: Protect the workspace from remote clone aliases

The provider MUST evaluate an intentional remote clone, but MUST reject the target
with an actionable operational error and no assessments when its scan path or a
traversed symlink reaches workspace content outside that clone, matches the file
identity of such content, cannot be resolved, or forms a symlink cycle.

#### Scenario: Remote clone points to another workspace file or directory

- **GIVEN** a remote clone contains a symlink into workspace content outside that clone
- **WHEN** the provider evaluates a scan path that would traverse the symlink
- **THEN** the provider rejects the target with an operational error
- **AND** it publishes no assessments for that target

#### Scenario: Remote scan path resolves outside the clone into the workspace

- **WHEN** a remote target selects a scan path that resolves into other workspace content
- **THEN** the provider rejects that target before conftest evaluation

#### Scenario: Non-cyclic internal clone symlink

- **GIVEN** a remote clone contains a non-cyclic symlink whose resolved target stays inside the clone
- **WHEN** the provider evaluates an otherwise eligible scan path containing it
- **THEN** the clone remains eligible for evaluation

#### Scenario: Symlink cycle inside the clone

- **GIVEN** a remote clone contains a symlink cycle such as a link to its own directory
- **WHEN** the provider inspects the scan path
- **THEN** traversal terminates and the target reports an operational error

### Requirement: Compare remote clones against workspace content outside them

The provider MUST omit paths under the selected clone while building the workspace
path and identity comparison sets, rather than subtracting identities afterwards. An
identity reachable at a workspace path outside the clone MUST still be detected.

#### Scenario: Clone file hard-linked to other workspace content

- **GIVEN** a file in the clone is hard-linked to a workspace file outside that clone
- **WHEN** the provider evaluates the clone
- **THEN** it rejects the target with an actionable operational error

#### Scenario: Ordinary remote clone inside the workspace

- **GIVEN** a remote clone contains ordinary files and no aliases outside it
- **WHEN** the provider evaluates the clone
- **THEN** its own files are not treated as workspace matches
- **AND** the clone produces its normal assessments

### Requirement: Preserve non-workspace target behavior

Subject to the alias, empty-input, and execution/output validation requirements above, the provider
MUST retain existing evaluation behavior for regular local files and local directories
outside the workspace and for remote repository targets. The workspace exclusion for
local targets MUST NOT exclude an intentional remote clone.

#### Scenario: Local file outside the workspace

- **WHEN** a target selects an ordinary local configuration file outside the workspace
- **THEN** the provider evaluates that file using its normal policy configuration

#### Scenario: Remote repository target

- **WHEN** a target selects a remote repository whose working clone is in the workspace
- **THEN** the provider evaluates the selected clone or scan path as before

### Requirement: Explain local workspace exclusion

OPA user documentation MUST describe which workspace directories are resolved and
excluded, every target rejection added by this change, and each rejection's remedy.

#### Scenario: User reads OPA documentation

- **WHEN** a user reads the OPA provider's local input guidance
- **THEN** it explains which local workspace content is excluded or rejected
- **AND** it explains the explicit-target, alias, hard-link, empty-input, conftest
  configuration, and multi-branch target rejections with their remedies
- **AND** it describes tested conftest versions without imposing an exact-version requirement

#### Scenario: User reads the documented limitations

- **WHEN** a user reads the same guidance
- **THEN** it explains that exclusion does not necessarily prevent directory traversal
- **AND** it explains that concurrent filesystem changes can defeat preflight isolation
- **AND** it explains that tests only establish behavior for versions actually exercised
