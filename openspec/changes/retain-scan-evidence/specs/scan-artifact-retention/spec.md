# Spec Delta

## Purpose

Preserve provider evaluation outputs across scans in discoverable locations so
users can inspect the artifacts referenced by historical Gemara Evaluation Logs.
Retention means local preservation, not a managed storage lifecycle. Providers
decide which preserved files to report as evidence; the Evaluation Log defines the
evidence set, while unreferenced files remain supporting outputs. Collection,
validation of collected copies, and optional packaging belong to a separate future
complyctl capability that must map moved artifacts without modifying the log.
Storage backends and long-term retention remain customer-owned; none are required
to use the evidence mapping delivered by this change.

## ADDED Requirements

### Requirement: Exclusive per-evaluation artifact storage

Providers MUST allocate a fresh directory beneath their provider results root for each
executed evaluation. The directory name MUST include a UTC timestamp and a unique
suffix. Allocation MUST NOT overwrite or reuse an existing evaluation directory,
including when timestamps or readable target labels collide.

#### Scenario: Three scans of the same target

- **WHEN** a provider evaluates the same target three times
- **THEN** three distinct evaluation directories contain their respective outputs
- **AND** subsequent scans leave previously retained artifact bytes unchanged

#### Scenario: Same timestamp and concurrent allocation

- **GIVEN** storage is writable and unique candidates are available within the retry limit
- **WHEN** two evaluations allocate output storage with identical timestamps and labels
- **THEN** each receives a distinct directory
- **AND** neither writes into the other's directory

#### Scenario: Existing candidate directory

- **GIVEN** a candidate evaluation directory already contains evidence
- **WHEN** allocation encounters that directory and a later candidate is available
- **THEN** allocation selects another unique suffix
- **AND** the existing directory and its files remain unchanged

#### Scenario: Allocation retries exhausted

- **WHEN** every candidate collides until the bounded retry limit is exhausted
- **THEN** allocation reports an operational error without publishing evidence
- **AND** all existing directories and files remain unchanged

#### Scenario: Independent evaluations in a single request

- **WHEN** a request includes repeated targets, multiple branches, or multiple Ampel specs
- **THEN** every executed target/branch/spec or target/branch evaluation has its own directory
- **AND** all assessments from one evaluation share that evaluation's artifacts

### Requirement: Defined storage roots and allocation naming

The provider results root MUST be `<workspace>/.complytime/<provider>/results`,
derived from existing workspace/provider constants. Each evaluation directory MUST
use `evaluation-<time>-<token>`, with UTC time formatted as `20060102T150405Z` and
at least 128 bits of cryptographic token entropy. Allocation MUST use atomic
no-replace creation with a bounded retry limit centralized in one documented
constant and overridable for deterministic tests.

#### Scenario: Fixed clock gives a defined workspace-relative coordinate

- **GIVEN** a fixed allocation clock and a test-controlled retry limit
- **WHEN** an OPA evaluation allocates its output directory
- **THEN** its coordinate starts with `.complytime/opa/results/`
- **AND** its evaluation component uses the documented UTC format and token
- **AND** a forced collision retries without overwriting existing files
- **AND** exhausting the configured test limit returns an operational error

### Requirement: Provider-owned allocation context

Provider initialization/configuration MUST supply the shared allocator with explicit
resolved workspace/results roots, derived from existing provider constants, and
verified ownership context from the launch-context adapter. Shared storage helpers
MUST validate confinement and MUST NOT import `cmd/*`, independently select roots,
interpret identity transport, or require mutable global result paths.

#### Scenario: Provider supplies storage context

- **GIVEN** Ampel, OpenSCAP, or OPA has resolved its workspace/results roots and verified owner
- **WHEN** its provider adapter invokes the shared allocator
- **THEN** allocation uses that explicit context and validates root confinement
- **AND** execution, parsing, hashing, and publication use the returned evaluation paths
- **AND** shared helpers do not import provider command packages or rediscover CWD

### Requirement: Constrained artifact access throughout evaluation

Providers MUST constrain allocation, writes, parsing, hashing, and ownership
finalization to the intended evaluation storage. They MUST reject symlinked context
components beneath the resolved provider results root, artifact symlinks, and
non-regular required outputs. Validated and hashed bytes MUST belong to the same
artifact; a pathname check followed by an unconstrained reopen is insufficient.

#### Scenario: Tool output is replaced by a symlink

- **GIVEN** a fresh evaluation contains a required output path
- **WHEN** that output is a symlink to a file outside the evaluation directory
- **THEN** the provider rejects the output before parsing, hashing, or changing its ownership
- **AND** it publishes no assessments or references backed by that output
- **AND** the external file remains unchanged

#### Scenario: Output is not a regular file

- **WHEN** a required output path is a directory or another non-regular file
- **THEN** the evaluation reports an operational error without publishing its evidence

#### Scenario: Workspace itself is reached through a symlink

- **GIVEN** the launcher supplies a workspace whose root path contains a symlink
- **WHEN** the provider resolves that root before evaluation
- **THEN** its resolved root is used consistently for constrained access and URL construction
- **AND** symlinked context components beneath its provider results root are still rejected

### Requirement: Evidence ownership follows the original invoking user

Providers MUST finalize each new evaluation directory and its files to the original
invoking user's verified UID/GID before publication. Normal
invocation keeps caller ownership; elevated-on-behalf execution restores that
user's ownership; direct root invocation remains root-owned. Providers MUST reject
unverified identities and MUST NOT guess ownership from arbitrary environment or
target values. Trusted identity delivery is an external prerequisite in design D9.

#### Scenario: Normal user invocation

- **GIVEN** operating-system identity identifies the current non-root caller
- **WHEN** an evaluation completes
- **THEN** its new directory and files remain owned by that caller's UID/GID
- **AND** no privileged ownership change is needed
- **AND** the caller can traverse the path, read the evidence, and verify its digest

#### Scenario: Elevated scan on behalf of a user

- **GIVEN** verified launch context identifies the original non-root caller
- **AND** the provider executes with elevated privileges
- **WHEN** tools finish and required outputs are closed, validated, and hashed
- **THEN** files are finalized to that caller's UID/GID before the directory is handed off
- **AND** present supporting metadata and required XCCDF results receive the same ownership
- **AND** the original caller can read and hash the reported evidence without elevation

#### Scenario: Direct root invocation

- **GIVEN** verified launch context identifies a direct root invocation without an originating user
- **WHEN** an evaluation completes
- **THEN** its directory and files remain owned by UID/GID 0

#### Scenario: Missing or invalid original-caller identity during elevation

- **GIVEN** elevated execution lacks a verified direct-root or original-caller identity
- **WHEN** the provider prepares to evaluate a target
- **THEN** the RPC returns an operational error before evaluation or ownership changes
- **AND** arbitrary `SUDO_UID`, `SUDO_GID`, or target values do not select an owner

### Requirement: Private evidence directory and file modes

Providers MUST use 0700 evaluation directories and 0600 files, including tool outputs
and metadata, and verify their modes and selected owner before publication. Failed
finalization MUST return an RPC error without a response. The conflicting repository
rule is not waived; maintainer reconciliation is required before implementation.

#### Scenario: Owner reads private finalized evidence

- **GIVEN** a completed normal or elevated-on-behalf evaluation
- **WHEN** its ownership and modes are finalized
- **THEN** the evaluation directory has mode 0700 and each file has mode 0600
- **AND** the selected original caller can traverse, read, and digest-verify the evidence
- **AND** metadata and required tool outputs have the same private access policy
- **AND** the provider does not add world or group permissions to avoid a failed handoff

### Requirement: Traversable context parents

The 0700 requirement applies to the evaluation leaf. Newly created context parents
MUST permit the selected owner to traverse the full path under the agreed
workspace-setup contract, without widening the leaf or changing unrelated existing
directories.

#### Scenario: New parents permit evidence access

- **GIVEN** evaluation allocation creates context parents
- **WHEN** the provider finalizes evidence for the selected owner
- **THEN** the owner can traverse those parents and read the private evaluation leaf
- **AND** unrelated existing directories remain unchanged

### Requirement: Ownership finalization is bounded and publication-critical

Providers MUST limit ownership changes to the fresh evaluation directory and its
contents, using constrained handles after tools stop writing. They MUST NOT
recursively chown existing workspace parents, inputs, clones, legacy files, or
earlier evaluations. Required evidence MUST be readable through its full path by
the selected owner under 0700/0600 modes. Finalization or traversal failures MUST
return an RPC error without publishing a response.

#### Scenario: Ownership handoff fails

- **WHEN** finalizing a required file's owner or verifying directory ownership fails
- **THEN** the RPC returns an operational error and publishes no scan response
- **AND** earlier evaluation ownership and bytes remain unchanged

#### Scenario: Existing parent prevents the owner from reading evidence

- **GIVEN** an existing parent directory cannot be traversed by the selected owner
- **WHEN** the evaluation is finalized
- **THEN** the RPC returns an actionable access error instead of publishing inaccessible evidence
- **AND** it does not change ownership of that existing parent

### Requirement: Readable and bounded evaluation context

Results MUST be grouped under readable target and evaluation-context path
components: repository/branch/spec for Ampel, target/branch for remote OPA,
target for local OPA, and target/profile for OpenSCAP. Labels MUST be non-empty
ASCII letter/digit/hyphen/underscore components of at most 80 bytes, replacing
other characters with hyphens and truncating as needed. Label collisions MUST NOT cause evidence
replacement or serve as the authority for artifact-instance identity.

#### Scenario: Find a target's policy-related results

- **WHEN** a user lists the results tree
- **THEN** repository or target labels and available branch/spec/profile labels are visible
- **AND** users can distinguish successive evaluations using timestamped child directories
- **AND** an Ampel collection spec is not misrepresented as a policy identifier

#### Scenario: Slash-containing branch and long spec path

- **GIVEN** supporting metadata is written successfully
- **WHEN** an evaluation uses branch `release/1` and a long nested spec path
- **THEN** each label occupies one bounded component beneath the provider results root
- **AND** the original non-secret values are available in evaluation metadata

#### Scenario: Lossy label collision

- **GIVEN** supporting metadata is written successfully
- **WHEN** different targets or branches produce identical readable labels
- **THEN** their evaluations remain separately stored and referenced
- **AND** metadata distinguishes their original target and branch values

#### Scenario: Unsafe destination

- **WHEN** a label or an existing symlink would redirect artifact creation outside the results root
- **THEN** the provider rejects the destination before writing evaluation artifacts

### Requirement: Retained provider output sets

Successful Ampel evaluations MUST retain both in-toto attestations. Successful
OpenSCAP evaluations MUST retain ARF and XCCDF results. Successful OPA evaluations
MUST retain conftest JSON output. Ampel and OPA MUST NOT write legacy normalized
result summary files, including error summaries, for new evaluations. Providers MUST retain failing compliance
outcomes as well as passing outcomes.

#### Scenario: Ampel evaluates two specs

- **WHEN** Ampel evaluates two specs against the same repository and branch
- **THEN** each evaluation retains `snappy.intoto.json` and `ampel.intoto.json`
- **AND** neither evaluation overwrites the other's attestations

#### Scenario: OpenSCAP produces a negative compliance result

- **WHEN** OpenSCAP completes with valid results reporting failed checks
- **THEN** the evaluation directory retains `arf.xml` and `results.xml`
- **AND** assessments reference that evaluation's ARF
- **AND** the unreferenced XCCDF result remains a supporting output, not implicit evidence

#### Scenario: OPA retains exact result output

- **WHEN** conftest completes successfully with valid JSON reporting passing or failing checks
- **THEN** `conftest.json` preserves the JSON output bytes used to construct assessments
- **AND** diagnostic stderr is not mixed into `conftest.json`

#### Scenario: Legacy summaries are not written for new evaluations

- **WHEN** an Ampel or OPA evaluation succeeds or fails
- **THEN** it does not write a new per-target normalized result summary
- **AND** any existing legacy summary remains unchanged

### Requirement: Uniform OPA result retention without input archives

OPA MUST apply the same result-file retention behavior to remote repositories,
local files, and local directories. Retention MUST NOT create historical clones
or copies of local input trees. The existing remote clone remains replaceable
working data rather than retained evidence.

#### Scenario: Remote input changes across scans

- **WHEN** OPA scans a remote branch three times and replaces its working clone
- **THEN** each evaluation's conftest result remains available
- **AND** retention creates no additional historical clone directories

#### Scenario: Local file changes after evaluation

- **WHEN** a user edits or deletes a local input file after OPA evaluates it
- **THEN** the retained conftest result and its evidence digest remain valid
- **AND** no retained copy of the input file is created

#### Scenario: Local directory is removed after evaluation

- **WHEN** a build process deletes the local directory OPA evaluated
- **THEN** evidence still resolves to the retained conftest JSON result
- **AND** retention has not copied the directory or its nested provider workspace

### Requirement: Retention preserves OPA input isolation

OPA MUST preserve the `opa-input-selection` contracts introduced by PR #233.
Retained artifacts and partial outputs MUST NOT become target input during enclosing
local directory scans. Explicit workspace targets and unsafe aliases MUST remain
rejected. The selected remote clone MUST remain eligible under the existing checks,
without allowing aliases into workspace content outside that clone. Output-root
selection MUST NOT remove host/provider input-exclusion boundaries when they differ.

#### Scenario: Repeated enclosing-directory scans accumulate evidence

- **GIVEN** a local directory contains eligible input and the provider workspace
- **WHEN** three scans create retained results and supporting metadata
- **THEN** only eligible non-workspace input contributes to each evaluation
- **AND** current/prior results, partial outputs, and other providers' artifacts remain excluded
- **AND** earlier retained evidence bytes and digests remain unchanged

#### Scenario: Retained evidence is selected through an alias

- **WHEN** local input or a remote clone outside the retained results contains an unsafe
  symlink or hard link to retained workspace evidence
- **THEN** the existing input-safety checks reject the affected configured target
- **AND** no evidence-backed assessments from that rejected target are published

#### Scenario: Existing private history cannot be inspected

- **GIVEN** an earlier private evaluation directory is inaccessible to OPA preflight
- **WHEN** workspace identities cannot be established reliably
- **THEN** OPA reports an actionable input-safety error and fails closed
- **AND** it does not skip historical identity checks, widen permissions, or change earlier owners

### Requirement: Evaluation context metadata

Each provider MUST attempt to write `metadata.json` for each completed evaluation
with `schema_version: 1`, evaluation token, provider, UTC collection-start timestamp,
target identity/key, and available configuration context. Metadata MUST exclude
credentials, artifact digests, target-variable maps, and environment variables.
Metadata is supporting information rather than evidence authority.

#### Scenario: Context behind a shortened label

- **GIVEN** supporting metadata is written successfully
- **WHEN** readable directory labels are truncated or collide
- **THEN** metadata records the original target ID and non-secret source path or URL
- **AND** records applicable branch/spec/profile and selected configuration context
- **AND** policy paths are described as provenance rather than retained policy snapshots

#### Scenario: Correlate repeated evaluations

- **GIVEN** metadata is successfully written for repeated evaluations of a target
- **WHEN** the same source is evaluated again
- **THEN** metadata records the same readable target key for that source
- **AND** branch, scan path, spec, or profile context distinguishes the evaluation where applicable
- **AND** the key is treated as a grouping hint, not proof of target identity

#### Scenario: Context matches the executed evaluation

- **GIVEN** supporting metadata is written successfully
- **WHEN** it describes an evaluation
- **THEN** it includes the evaluation token, provider, collection timestamp, target identity,
  and available branch, spec, profile, policy path, content key, requirement IDs, or namespaces
- **AND** it omits artifact digests and unavailable fields
- **AND** its top-level keys are limited to `schema_version`, `evaluation_id`,
  `provider`, `collected_at`, `target`, and `configuration`

#### Scenario: Credentials in target variables

- **WHEN** a target includes an access token or credential-bearing URL
- **THEN** provider-generated labels and metadata do not persist those credentials
- **AND** the provider does not serialize the entire target-variable map

#### Scenario: Credentials appear in configuration URLs or reference context

- **WHEN** target or configuration context contains URL userinfo, query, or fragment values
- **THEN** metadata, labels, mapping titles, and descriptions omit those URL components
- **AND** arbitrary target/configuration maps and environment values are not serialized

### Requirement: Target acceptance precedes OPA evidence publication

OPA MUST preserve target rejection from `opa-input-selection`: input-safety/preflight
rejection, including preflight cancellation, or unconfirmed directory input in any
remote branch MUST withhold all branches' assessments, evidence, and references for
that configured target. It MUST retain operational errors and other targets' accepted
results. Retention MUST NOT broaden ordinary branch-scoped processing failures.

#### Scenario: Successful retained branch precedes a rejected branch

- **GIVEN** one remote branch has written complete retained output
- **AND** another selected branch fails input-safety preflight or lacks confirmed directory input
- **WHEN** the provider constructs the response in either branch order
- **THEN** it publishes no assessments, evidence entries, or mapping references for that target
- **AND** previously written branch artifacts remain on disk without published references
- **AND** target/branch operational errors and other targets' accepted evidence remain

#### Scenario: Separated diagnostic output reports no eligible directory input

- **WHEN** conftest reports empty effective directory input on stderr or stdout lacks
  structured records confirming evaluated directory input
- **THEN** OPA preserves the existing input-selection error classification and actionable remedy
- **AND** diagnostic stderr is not mixed into retained `conftest.json`
- **AND** no synthetic passing assessment or evidence reference is published for the rejected target
- **AND** unrelated execution failures remain distinguishable from empty-input errors

### Requirement: No publication of incomplete evidence

Providers MUST report allocation, required artifact write/close, parse, digest, or
ownership-finalization failures as operational errors. They MUST NOT publish assessments or mapping
references backed by incomplete evidence for the affected evaluation, substitute
an earlier artifact, or mutate previously completed evaluations. Unreferenced
partial outputs MAY remain for diagnosis.

#### Scenario: Disk failure while saving OPA output

- **WHEN** writing or closing `conftest.json` fails
- **THEN** the evaluation reports an operational error
- **AND** it publishes no assessment backed by that incomplete file

#### Scenario: Missing or invalid tool output

- **WHEN** a tool fails to produce the expected output or produces invalid result content
- **THEN** the provider does not read a previous evaluation's output as a fallback
- **AND** the failed evaluation's partial artifacts are not referenced as valid evidence

#### Scenario: OpenSCAP evaluation fails operationally

- **WHEN** allocation, required output, parsing, hashing, or ownership finalization fails
- **THEN** the Scan RPC returns a non-nil error and no response

#### Scenario: Ampel per-evaluation output processing fails

- **WHEN** a target/branch/spec evaluation fails allocation, required writes/closes, or parsing
- **THEN** `ScanResponse.Errors` identifies the failed target/evaluation
- **AND** no assessments or references are published for that evaluation
- **AND** unrelated successful evaluations may remain in the response

#### Scenario: Ampel digest processing fails

- **WHEN** hashing an Ampel evidence artifact fails
- **THEN** the Scan RPC returns a non-nil error and no response

#### Scenario: OPA per-evaluation processing fails

- **WHEN** a target evaluation fails allocation, required writes/closes, parsing, or hashing
- **THEN** `ScanResponse.Errors` identifies the failed target/evaluation
- **AND** no assessments or references are published for that evaluation
- **AND** unrelated successful evaluations may remain in the response

#### Scenario: Common workspace, identity, or finalization fails

- **WHEN** required workspace/identity context validation or ownership finalization fails
- **THEN** the Scan RPC returns a non-nil error and no response for any provider

#### Scenario: Supporting metadata cannot be written

- **GIVEN** the required outputs are valid and evidence digests have been computed
- **WHEN** writing or closing supporting metadata fails
- **THEN** the provider's injectable logger records a warning identifying the evaluation token
- **AND** otherwise complete evidence is published after successful ownership finalization
- **AND** metadata may be missing or incomplete and is not an evidence authority
- **AND** earlier evaluations remain readable and unchanged

### Requirement: Preserve existing artifacts without automatic cleanup

New scans MUST leave legacy output files and completed evaluation directories
untouched. This feature MUST NOT delete retained evidence, introduce retention
settings, write moving fixed-path aliases, or deduplicate historical outputs.
Replacing OPA working clones MUST remain distinct from evidence retention.

#### Scenario: Upgrade with existing evidence

- **WHEN** a scan runs in a workspace containing legacy fixed-path artifacts
- **THEN** it writes the new evaluation layout without migrating or replacing those artifacts
- **AND** existing Evaluation Logs are not rewritten

#### Scenario: Identical results on successive scans

- **WHEN** two evaluations produce identical output bytes
- **THEN** each has its own retained output set
- **AND** no automatic cleanup or content deduplication occurs
