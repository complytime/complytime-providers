# Spec Delta

## MODIFIED Requirements

### Requirement: Evidence entry per assessment log

Each provider MUST populate at least one `Evidence` entry on every
`AssessmentLog` it returns in a `ScanResponse`.

#### Scenario: OpenSCAP returns ARF evidence

- **GIVEN** OpenSCAP has completed an evaluation and retained a valid ARF file
- **WHEN** the provider constructs the `ScanResponse`
- **THEN** every assessment from that evaluation contains evidence with type `ARF`
- **AND** a non-empty RFC 3339 `CollectedAt` timestamp

#### Scenario: Ampel returns two attestation evidence entries

- **GIVEN** Ampel has successfully scanned a repository/branch/spec combination
- **WHEN** the provider constructs the `ScanResponse`
- **THEN** assessments from that evaluation contain evidence for both retained attestations
- **AND** their IDs use the `ampel_` and `snappy_` prefixes and the evaluation identity
- **AND** both have type `IntotoAttestation` and non-empty RFC 3339 `CollectedAt` timestamps

#### Scenario: OPA returns conftest evidence

- **GIVEN** OPA has completed an evaluation of a remote repository, local file, or local directory
- **WHEN** the provider constructs the `ScanResponse`
- **THEN** every assessment from that evaluation contains evidence for retained conftest JSON
- **AND** the type remains `ConftestResult` with a non-empty RFC 3339 `CollectedAt` timestamp

### Requirement: EvidenceSource coordinate references a file path

Each `Evidence` entry MUST have a non-nil `Source` with `Coordinate` set to the
retained evidence artifact's filesystem path relative to the workspace root, as
defined in `Workspace-relative evidence references`. The path MUST be the same artifact
used to construct the assessment, not a reconstructed fixed filename or a
replaceable input location.

#### Scenario: OpenSCAP coordinate points to ARF file

- **GIVEN** a completed OpenSCAP scan with a retained ARF file
- **WHEN** an OpenSCAP evidence entry is returned
- **THEN** `Source.Coordinate` identifies `arf.xml` in that evaluation's directory

#### Scenario: Ampel coordinates point to attestation files

- **GIVEN** a completed Ampel repository/branch/spec evaluation
- **WHEN** its evidence entries are returned
- **THEN** their coordinates identify `ampel.intoto.json` and `snappy.intoto.json`
  in that evaluation's directory

#### Scenario: OPA coordinate points to retained evaluation output

- **GIVEN** a completed OPA evaluation against any supported input type
- **WHEN** an OPA evidence entry is returned
- **THEN** `Source.Coordinate` identifies `conftest.json` in that evaluation's directory
- **AND** it does not identify the working clone, local input, or normalized summary

#### Scenario: OPA coordinate points to evaluated input

Legacy-log compatibility only: this baseline scenario name is retained for OpenSpec
delta matching. New evaluations reference preserved output, not input.

- **GIVEN** a legacy Evaluation Log whose OPA coordinate points to evaluated input
- **WHEN** the upgraded provider performs a new evaluation
- **THEN** it leaves the legacy log unchanged
- **AND** the new evaluation's coordinate identifies retained conftest output instead

#### Scenario: Evidence survives source replacement

- **GIVEN** an Evaluation Log referencing a completed OPA evaluation
- **WHEN** another scan replaces the clone or a user edits or removes the local input
- **THEN** the previous evidence coordinate still resolves to its original conftest output

### Requirement: EvidenceSource digest for file-based evidence

Every retained evidence artifact in these providers is a file. `Source.Digest`
MUST contain its SHA256 hash in `sha256:<hex>` format, computed from the stored
bytes before evidence publication. OPA MUST hash the retained result file
regardless of whether its evaluated input was a file or directory.

#### Scenario: OpenSCAP digest covers ARF file

- **GIVEN** a completed OpenSCAP evaluation with a retained ARF file
- **WHEN** an OpenSCAP evidence entry is returned
- **THEN** its digest matches `sha256:[a-f0-9]{64}` and the retained ARF bytes

#### Scenario: Ampel digest covers attestation files

- **GIVEN** a completed Ampel evaluation with both retained attestations
- **WHEN** its evidence entries are returned
- **THEN** each digest matches `sha256:[a-f0-9]{64}` and its corresponding stored file

#### Scenario: OPA digest covers results for every input type

- **WHEN** OPA returns evidence for a remote repository, local file, or local directory
- **THEN** its digest matches `sha256:[a-f0-9]{64}` and the stored `conftest.json` bytes
- **AND** no input-file hash or empty directory digest substitutes for that result-file hash

#### Scenario: OPA omits digest for directory inputs

Legacy-log compatibility only: the unchanged scenario name does not exempt new
evaluations from the retained-result digest requirement.

- **GIVEN** a legacy OPA Evaluation Log omitting a digest for directory input evidence
- **WHEN** the upgraded provider evaluates that directory again
- **THEN** it leaves the legacy log unchanged
- **AND** the new evidence includes the digest of the retained conftest result file

#### Scenario: Digest computation failure returns an error

- **GIVEN** an evidence artifact is missing, unreadable, or cannot be hashed completely
- **WHEN** the provider attempts to compute its digest
- **THEN** it reports an operational error and publishes no evidence-backed assessment
  for the affected evaluation

### Requirement: EvidenceSource ReferenceID links to MappingReference

Each `Evidence.Source.ReferenceID` MUST match a `MappingReference.ID` in the same
response. References and evidence IDs MUST distinguish artifact instances using
provider-specific artifact kind and evaluation identity. A mapping reference's URL
MUST be a properly escaped absolute `file://` URL identifying the same retained
file as the coordinate resolved against the workspace root. Repeated use of the
same artifact across assessments MUST share its identity and mapping reference.

`Evidence.ID` is provider-internal and need not equal `MappingReference.ID`.
Only `Evidence.Source.ReferenceID` MUST equal its `MappingReference.ID`.
Evidence IDs MUST use `ampel_<token>`, `snappy_<token>`, `arf_<token>`, or
`conftest_<token>` for their respective artifact kinds. Mapping-reference IDs MUST
use `ampel_<token>`, `snappy_<token>`, `openscap-arf_<token>`, or
`conftest-result_<token>`, respectively, with the allocated evaluation token.

#### Scenario: OpenSCAP declares ARF mapping reference

- **WHEN** OpenSCAP returns evidence for a completed evaluation
- **THEN** its mapping reference uses an evaluation-qualified `openscap-arf` identity
- **AND** every ARF source reference matches that identity and retained artifact URL

#### Scenario: Ampel declares attestation mapping references

- **WHEN** Ampel returns evidence from one or more completed evaluations
- **THEN** each attestation's source reference matches its mapping-reference ID
- **AND** the reference URL identifies that attestation's retained file

#### Scenario: OPA declares result mapping reference

- **WHEN** OPA returns evidence from a completed evaluation
- **THEN** the source reference matches a result-file mapping reference
- **AND** its title describes conftest results and its URL identifies retained `conftest.json`

#### Scenario: OPA declares input mapping reference

Legacy-log compatibility only: the unchanged scenario name describes historical
input references; new evaluations declare result-file references.

- **GIVEN** a legacy OPA Evaluation Log declaring an input mapping reference
- **WHEN** the upgraded provider evaluates the target again
- **THEN** it leaves the legacy reference unchanged
- **AND** the new response declares a separate result-file mapping reference

#### Scenario: Repeated targets and colliding display labels

- **WHEN** distinct evaluations share a target, timestamp, or sanitized display label
- **THEN** their artifact identities and references remain distinct
- **AND** deduplication by reference ID does not collapse different artifacts

#### Scenario: Many assessments share one artifact

- **WHEN** several assessments derive from one evaluation's ARF or conftest output
- **THEN** they share the corresponding artifact identity and mapping reference
- **AND** the provider does not make an artifact copy per assessment

#### Scenario: Retained remote branch belongs to a rejected configured target

- **GIVEN** a remote branch produced retained conftest output
- **AND** any selected branch triggers configured-target input-selection rejection
- **WHEN** OPA aggregates the response
- **THEN** no evidence entry or mapping reference for any branch of that target is returned
- **AND** written artifacts remain unreferenced rather than becoming implicit evidence
- **AND** other configured targets retain their accepted evidence/reference pairs

### Requirement: Man page documents evidence artifacts

Each provider's man page MUST include an `EVIDENCE` section describing retained
artifacts, their production, their representation in the Evaluation Log, and
the preservation guarantee. It MUST explain that output retention means local
preservation across scans, is always on, and has no automatic cleanup in
this feature. It MUST identify the Gemara Evaluation Log as the source of truth for
the evidence set and distinguish reported evidence from unreferenced supporting
outputs. It MUST explain that collection, validation of collected copies, and
optional packaging are a separate future complyctl capability, while storage and
long-term retention remain customer-owned.

#### Scenario: OpenSCAP man page describes ARF evidence

- **WHEN** a user reads the OpenSCAP man page
- **THEN** it describes per-evaluation ARF storage, production by `oscap xccdf eval`,
  coordinates, digests, and instance-qualified references

#### Scenario: Ampel man page describes attestation evidence

- **WHEN** a user reads the Ampel man page
- **THEN** it describes both retained attestations, readable repo/branch/spec grouping,
  and their Evaluation Log representation

#### Scenario: OPA man page describes conftest evidence

- **WHEN** a user reads the OPA man page
- **THEN** it describes retained conftest JSON evidence for both local and remote targets
- **AND** explains that original inputs and policies are not archived and historical
  re-evaluation is not guaranteed
- **AND** explains that unchanged findings do not prove the evaluated input was unchanged
- **AND** distinguishes embedded conftest input filenames from retained evidence coordinates

#### Scenario: Owner reads evidence from an elevated scan

- **WHEN** a user reads the provider's evidence documentation
- **THEN** it explains execution-account ownership, private modes and directory traversal needs
- **AND** it states that sudo and direct-root scans keep root-owned evidence
- **AND** it describes authorized elevated reading/hashing without promising original-user access

### Requirement: Expanded FILES section in man pages

Each provider's man page `FILES` section MUST list concrete artifact path patterns,
including readable context directories, timestamped evaluation directories, and
`metadata.json`, as well as existing Generate-time and working-data locations.

#### Scenario: OpenSCAP FILES lists scan artifacts

- **WHEN** a user reads the OpenSCAP man page
- **THEN** it lists retained `arf.xml`, `results.xml`, and metadata paths, plus
  existing tailoring and remediation paths

#### Scenario: Ampel FILES lists attestation artifacts

- **WHEN** a user reads the Ampel man page
- **THEN** it lists both attestation paths, optional supporting metadata,
  and the existing generated scan configuration

#### Scenario: OPA FILES lists scan artifacts

- **WHEN** a user reads the OPA man page
- **THEN** it lists retained `conftest.json` and optional supporting metadata paths,
  scan configuration, mapping, and generated directories
- **AND** identifies the clone directory as replaceable working data

#### Scenario: Legacy normalized summaries are documented as discontinued

- **WHEN** a user reads Ampel or OPA artifact documentation
- **THEN** it states legacy normalized summaries are no longer written
- **AND** existing summary files remain untouched and are not current scan results

## ADDED Requirements

### Requirement: Evaluation Log defines the evidence set

Providers MUST explicitly report the files selected as evidence through the
`Evidence` entries and linked mapping references carried into the Gemara Evaluation
Log. Every reported evidence artifact MUST be preserved and have a coordinate,
digest, and matching mapping reference. A file MUST NOT be treated as reported
evidence solely because it is stored beside an evidence artifact. Supporting
outputs MUST NOT implicitly extend the evidence set defined by the log.

#### Scenario: Supporting outputs accompany reported evidence

- **GIVEN** an OpenSCAP evaluation preserves ARF, XCCDF results, and supporting metadata
- **WHEN** the provider reports only the ARF as evidence in the Evaluation Log
- **THEN** the log's evidence set includes the ARF, not the unreferenced supporting files
- **AND** retaining supporting files does not require adding implicit evidence entries

#### Scenario: Historical log identifies every reported artifact

- **GIVEN** a completed evaluation with evidence recorded in a Gemara Evaluation Log
- **WHEN** later scans evaluate the same targets
- **THEN** every evidence entry in the earlier log still identifies its original preserved file
- **AND** each file matches the earlier log's recorded digest without packaging or uploads

### Requirement: Consistent evaluation collection timestamp

Providers MUST capture one UTC collection-start timestamp per evaluation before
tool execution. The evaluation directory's timestamp component, metadata when
written, and all evidence `CollectedAt` fields
for that evaluation MUST derive from that instant. Uniqueness MUST NOT depend
on clock precision or ordering.

#### Scenario: Evaluation crosses a clock boundary

- **WHEN** an evaluation starts before a second or day boundary and finishes afterward
- **THEN** its directory timestamp and evidence collection timestamps represent its start
- **AND** parsing time does not replace the recorded collection timestamp

#### Scenario: Clock moves backward

- **WHEN** a later evaluation observes an equal or earlier wall-clock time
- **THEN** it still receives a distinct artifact location and identity
- **AND** no monotonic ordering or global complyctl scan ID is inferred from the timestamp

### Requirement: Workspace-relative evidence references

Evidence coordinates MUST be filesystem paths relative to complyctl's resolved
workspace root containing `.complytime`. Mapping-reference URLs MUST remain escaped
absolute `file://` URLs identifying the same artifacts. Documentation MUST explain
coordinate resolution against the selected root, not the log's directory or the
consumer's current directory. The evidence owner MUST be able to locate and
digest-verify evidence in the originating workspace without packaging.

#### Scenario: In-place evidence resolution

- **GIVEN** an Evaluation Log and its evidence remain in the originating workspace
- **WHEN** the evidence owner resolves a coordinate against that root and reads its absolute file URL
- **THEN** both identify the same retained artifact
- **AND** the artifact's digest matches the Evaluation Log
- **AND** resolution does not depend on the log's directory or the auditor's current directory
- **AND** no collection, package, or storage integration is required

#### Scenario: Copied workspace does not relocate absolute URLs

- **GIVEN** a log and its referenced workspace tree are copied to a different root
- **WHEN** a consumer resolves a coordinate against the copied workspace root
- **THEN** the coordinate identifies the copied artifact with the recorded digest
- **AND** the unchanged absolute file URL still describes the originating filesystem
- **AND** this change does not promise automatic URL relocation or implement packaging

#### Scenario: URL contains special characters

- **WHEN** the retained workspace path contains spaces, percent signs, or non-ASCII characters
- **THEN** the absolute file URL is properly escaped
- **AND** decoding it identifies the same file as the coordinate resolved against the workspace root

### Requirement: Output roots remain separate from OPA exclusion boundaries

OPA MUST retain host `.complytime` and configured provider-artifact boundaries for
input exclusion, including when they differ. Using the existing `workspace` variable
for output-root resolution MUST NOT remove dual-boundary or alias checks.

#### Scenario: Coordinate root and exclusion boundaries differ

- **GIVEN** the host-injected workspace base is B and the configured provider artifact directory differs
- **WHEN** OPA scans eligible enclosing input under its existing safety checks
- **THEN** retained output and evidence coordinates use B as their root
- **AND** input selection protects both B's `.complytime` and the configured provider artifact directory

### Requirement: Provider workspace root uses existing workspace variables

Providers MUST resolve Generate global/Scan target `workspace` variables to an
absolute root for generated artifacts and evidence coordinates. Absent/empty values
MUST fall back to process CWD. Invalid non-empty roots MUST fail the RPC before
evaluation. Providers MUST NOT reapply host flag/environment precedence or require
launcher CWD changes, new RPC fields, or process-wide chdir.

#### Scenario: Non-default workspace

- **GIVEN** unchanged complyctl supplies workspace B through existing RPC variables
- **AND** the provider process CWD is a different directory A
- **WHEN** the provider runs Generate and Scan
- **THEN** it writes generated artifacts and evidence beneath B's `.complytime`
- **AND** every evidence coordinate resolves against B to its preserved artifact
- **AND** every absolute file URL identifies that same artifact with the recorded digest
- **AND** the providers do not write their generated artifacts or evidence beneath A

#### Scenario: Environment does not override the supplied workspace variable

- **GIVEN** the RPC workspace variable names B and COMPLYTIME_WORKSPACE names A
- **WHEN** the provider initializes the evaluation workspace
- **THEN** it uses B as the coordinate base and generated-artifact root
- **AND** it does not reinterpret complyctl's workspace-selection precedence

#### Scenario: Workspace root cannot be resolved

- **WHEN** resolving a supplied non-empty workspace root fails
- **THEN** the RPC returns an operational error before evaluation or evidence publication
- **AND** the provider does not fall back to process CWD

#### Scenario: Legacy caller omits the workspace

- **GIVEN** workspace is absent or empty and the provider process CWD is A
- **WHEN** Generate or Scan resolves its root
- **THEN** it uses A without requiring new launcher behavior

### Requirement: Consistent request workspace

Providers MUST resolve one workspace per RPC before storage access. Scan targets
MUST resolve to the same root. Generate global workspace MUST be authoritative;
a non-empty target workspace MUST agree with it. Conflicts MUST return an RPC error.

#### Scenario: Scan targets disagree on workspace

- **GIVEN** a request contains targets resolving to different workspace roots
- **WHEN** Scan validates the request
- **THEN** it returns an RPC error before evaluation or storage access

#### Scenario: Generate target disagrees with global workspace

- **GIVEN** Generate global workspace is B and target workspace resolves to A
- **WHEN** the provider validates the request
- **THEN** it returns an RPC error before storage access

### Requirement: Workspace-relative input and configuration resolution

Providers MUST resolve relative input/configuration paths against the selected
workspace and leave absolute paths absolute. Tool subprocess CWD and CWD-sensitive
configuration inspection MUST use that same workspace without process-wide chdir.
Documentation MUST identify this as a breaking provider behavior for configurations
that previously relied on invocation-relative paths.

#### Scenario: Invocation and workspace contain different inputs

- **GIVEN** process CWD is A and the supplied workspace is B
- **AND** A and B contain different `manifests/` inputs
- **WHEN** OPA evaluates `input_path: manifests/`
- **THEN** it evaluates B's input without modifying A's input
- **AND** conftest configuration discovery uses B rather than A

#### Scenario: Relative explicit conftest configuration

- **GIVEN** the workspace is B and CONFTEST_CONFIG_FILE is a relative path
- **WHEN** OPA determines ignore rules and executes conftest
- **THEN** both resolve the configuration against B with consistent precedence
- **AND** ambiguity retains the existing fail-closed behavior

#### Scenario: Absolute external input

- **GIVEN** a valid absolute input path outside the workspace
- **WHEN** the provider resolves that input
- **THEN** it keeps the absolute path and applies existing input-safety checks
