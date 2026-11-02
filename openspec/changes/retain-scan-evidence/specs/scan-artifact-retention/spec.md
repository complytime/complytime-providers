# Spec Delta

## Purpose

Preserve provider evaluation outputs across scans in discoverable locations so
users can inspect the artifacts referenced by historical Evaluation Logs.

## ADDED Requirements

### Requirement: Exclusive per-evaluation artifact storage

Providers MUST allocate a fresh directory beneath their results root for each
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

### Requirement: No publication of incomplete evidence

Providers MUST report allocation, required artifact write/close, parse, or digest
failures as operational errors. They MUST NOT publish assessments or mapping
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

#### Scenario: Supporting metadata cannot be written

- **GIVEN** the required outputs are valid and evidence digests have been computed
- **WHEN** writing or closing supporting metadata fails
- **THEN** the provider logs a warning and publishes the completed evaluation
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
