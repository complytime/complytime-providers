# Design

## Context

See [proposal.md](proposal.md) for motivation and scope. The existing implementation
already carries evidence fields through the complyctl SDK into the Evaluation Log;
the evidence mapping itself does not need an RPC extension. Delivery of trusted
launch context is an external integration prerequisite, not an existing SDK claim.

- Ampel constructs attestation paths in `scan.ScanRepository`, then hashes them in
  `server.Scan`. Its summary writer currently omits spec identity from the filename.
- Ampel and OPA write normalized result summaries. Ampel's historical Export RPC
  consumed these files, but the current SDK and providers no longer expose Export;
  external scripts may still rely on the documented files.
- OpenSCAP writes and reads global `config.ARFPath` and `config.ResultsPath`.
- OPA's `GitLoader.Load` removes an existing repo/branch clone before cloning;
  it leaves the current clone on disk after evaluation. Local input is evaluated
  in place. `evalAndParse` hashes local file input and `buildEvidence` references
  input paths. Raw conftest output is parsed but not separately retained.
- OPA's command runner currently uses combined stdout/stderr. A retained JSON
  result needs stdout separated from diagnostics.
- PR #233 is merged as `10af2fd`. Its `opa-input-selection` contracts and
  `target-scoped-generation` aggregation delta are the implementation baseline:
  workspace exclusion, alias checks, confirmed evaluated directory input, and
  configured-target rejection remain in force when output retention is introduced.
- The prior evidence-source design chose target-stable, readable IDs. It documented
  lossy encoding, now tracked by #199. Retained artifact instances need their own
  identity without making filesystem allocation depend on that encoding.
- `ScanRequest` supplies targets but no general policy name, policy ID, or global
  scan ID. Generate-time config supplies policy paths, requirements, namespaces,
  and optionally content keys. Collection specs are not policy identities.

The `evidence-source` baseline is also present in the tracked archive at
`openspec/changes/archive/2026-10-27-populate-evidence-source/specs/evidence-source/spec.md`.
At proposal creation, the synchronized main spec is an existing untracked file;
this change does not take ownership of that unrelated working-tree content.

## Goals / Non-Goals

**Goals:**

- Separate readable logical context, unique evaluation identity, and artifact paths.
- Write tool outputs directly into an evaluation-owned directory and pass those
  actual paths through parsing, hashing, and reference construction.
- Give OPA local and remote targets an identical result-preservation guarantee
  without storing input copies.
- Reuse the existing plugin and result-mapping architecture with shared allocation,
  naming constants, and small metadata primitives in `internal/evidence`.
- Make every reported evidence artifact locatable and digest-verifiable from the
  Gemara Evaluation Log in the originating workspace without packaging or uploads.

**Non-Goals:**

- General logical-ID encoding repair (#199), input snapshots, policy archives,
  directory hashing, filesystem deduplication, or an evidence database.
- Reworking OpenSCAP target dispatch or mutable Generate-time configuration.
- Claiming full concurrent-scan safety for OPA's shared clone/policy workspaces.
  Artifact allocation is concurrency-safe; input acquisition races remain a
  separate concern and must not be disguised by the retention documentation.
- Implementing cleanup, including a new post-scan clone deletion lifecycle.
- Collecting evidence sets, validating collected copies, or designing an export
  package, relocation index, storage backend, or managed retention lifecycle.

## Responsibility boundary

Retention in this change means local preservation across scans. The Gemara
Evaluation Log is the source of truth for the evidence set. Providers select the
files they report as evidence and preserve their bytes, coordinates, digests, and
reference identities. A file's presence in an evaluation directory does not make
it evidence: OpenSCAP's XCCDF result and optional metadata remain supporting outputs
unless explicitly reported as evidence. The output requirements in D1 remain intact.

A separate future complyctl core feature can collect, validate, and optionally
package every artifact referenced as evidence in the log. It must use the log's
evidence mappings rather than infer evidence from directory contents. If packaging
moves files, that feature must map the moved artifacts to the original references
without modifying the Evaluation Log. The format and mechanism are outside this
change, and packaging is not required for the default local paper trail.

Customer integrations own storage backends, transfers, credentials, access policies,
archival, and longevity. Customers may keep evidence locally without any integration.
This contract enables future collection work; it does not require that work to be
designed or implemented before providers preserve and map their evidence.

## Decisions

### D1: Group by readable context; allocate one directory per evaluation

The workspace root is the absolute provider process working directory supplied by
the launcher. The provider results root is
`<workspace>/.complytime/<provider>/results`; an evaluation directory is one fresh
leaf beneath that results root. Coordinates include `.complytime/` and are relative
to the workspace root. Derive these paths from `provider.WorkspaceDir` and existing
provider directory constants rather than duplicating the workspace name in logic.

Provider initialization/configuration owns resolving the workspace root and deriving
its absolute results root from existing provider constants. Ampel's relative path
helpers, OpenSCAP's global result paths, and OPA's injected artifact root are adapted
at their provider boundaries; retention paths are passed explicitly, not installed
in mutable globals. A provider-side launch-context adapter establishes the selected
verified owner according to the companion contract in D9.

The shared allocator accepts explicit resolved workspace/results roots, verified
ownership context, and evaluation labels. It validates root confinement and owns
allocation, constrained artifact access, and finalization. It MUST NOT import
`cmd/*`, rediscover CWD, select workspace precedence, interpret identity transport,
or duplicate provider-specific directory constants. Its returned evaluation value
carries the allocated directory, token, collection time, and artifact access needed
by execution, parsing, hashing, and response construction. Provider orchestration
owns tool execution and result interpretation; shared helpers own storage mechanics.

The allocator's input contract comprises the two resolved roots, evaluation labels,
and a verified owner; its result comprises the allocated directory, token, collection
time, and constrained artifact operations. Provider adapters extract launch context
and supply provider-specific roots. Once the companion identity contract is agreed,
share its provenance validation and rejection rules across adapters rather than
implementing independent trust decisions for each provider. The allocator consumes
the validation result; it does not treat an arbitrary UID/GID pair as proof of trust.
Names and signatures remain implementation details until that contract is established.

Use the following workspace-relative patterns. Each label is one filesystem
component, not an untrusted path. Local OPA and OpenSCAP use `TargetID` when
available, with a source-derived or `system` fallback. Remote labels identify the
repository host and path; branches and Ampel spec references have their own labels.

```text
.complytime/ampel/results/<repository>/<branch>/<spec>/evaluation-<time>-<token>/
  snappy.intoto.json
  ampel.intoto.json
  metadata.json

.complytime/openscap/results/<target>/<profile>/evaluation-<time>-<token>/
  arf.xml
  results.xml
  metadata.json

.complytime/opa/results/<repository>/<branch>/evaluation-<time>-<token>/
  conftest.json
  metadata.json

.complytime/opa/results/<local-target>/evaluation-<time>-<token>/
  conftest.json
  metadata.json
```

Allocate once per Ampel target/branch/spec execution, per OPA local target or
remote target/branch execution, and per currently executed OpenSCAP system scan.
Do not allocate per assessment or assume one RPC equals one evaluation. Requests
can evaluate the same logical target repeatedly.

Human-readable labels are display grouping only. Use a filesystem-label helper
separate from `SanitizeForID`: ASCII letters, digits, hyphens and underscores;
replace other characters with hyphens, cap each label at 80 bytes, and provide a
non-empty fallback. Strip URL userinfo, query, and fragment before deriving labels.
Attempt to preserve non-secret context in metadata. Label collisions can share a
parent, but must never share an evaluation directory.

Use root-constrained filesystem operations for allocation, writes, parsing, hashing,
and ownership finalization. Resolve the workspace root once, including a symlinked
workspace root, then reject symlinked context components beneath the provider
results root. Reject artifact symlinks and non-regular outputs before parsing or
hashing; use constrained handles rather than a pathname check followed by an
unconstrained reopen. Confirm validated and hashed bytes belong to the same artifact.

The author has selected private evaluation directories (0700) and files (0600),
including tool outputs and metadata, with original-caller ownership per D9. This is
the evidence-access product decision, not an open mode choice. Centralize these
values, label limits, time format, and basenames in focused allocation/metadata
files; leave pure ID helpers free of filesystem I/O. The conflicting convention-pack
instruction is reported separately under `Repository instruction conflicts`; this
spec records the selected design without changing or waiving that instruction.
The 0700 requirement applies to the evaluation leaf, not automatically to its context
parents. Newly created parents MUST permit the selected owner to traverse the full
path; parent ownership and access are subject to the companion workspace-setup
contract. An inaccessible parent is an error, not a reason to widen the evaluation
leaf or alter unrelated existing directories.

**Alternatives:** Flat opaque directories obscure target/spec context. Suffixing
every existing filename repeats naming logic and keeps long unsafe branch/spec
components. Deep hierarchy without an exclusively allocated evaluation leaf still
allows collisions and overwrites.

### D2: UTC time is context, not uniqueness

Capture one UTC time at evaluation allocation, before tool execution. Format the
directory component as `20060102T150405Z`; emit the same instant as RFC 3339 in
metadata when written and evidence collection fields. Define it as collection start,
not tool completion or an attestation's internal timestamp.

Use a standard-library cryptographically random token with at least 128 bits of
entropy. Atomically create the evaluation directory without replacement. Retry an
existing-name collision with a fresh token using a bounded shared retry limit,
centralized in one documented constant with a deterministic test override;
return an operational error on exhaustion or any other allocation failure. Do not
use an existence-check-then-create sequence. Pass an evaluation value carrying
time, token, and allocated directory to provider-specific operations; avoid globals.

The artifact-instance identity uses the provider/artifact-specific prefix from D4
and the token; no additional provider field is implied by this wording. Thus
distinct evaluations do not collapse when labels or timestamps match. Filesystem
exclusive creation protects existing evidence even if a candidate name collides.
The token is generated independently of target labels and is not a globally
coordinated complyctl scan identifier.

**Alternatives:** Seconds or nanoseconds alone cannot guarantee uniqueness across
processes, coarse clocks, or clock rollback. Process counters reset on restart.
No new UUID dependency or persistent index is needed.

### D3: OPA retains conftest output, not inputs

For all input types, persist the conftest JSON stdout bytes as `conftest.json` in
the evaluation directory, parse those same bytes, and hash the stored file. The
coordinate and mapping-reference URL identify that file. Keep type `ConftestResult`.

```text
replaceable remote clone ─┐
                         ├─ conftest ─ stdout JSON ─ conftest.json ─ digest + source
original local input ────┘                   │
                                            └─ parse ─ assessments
```

Adjust OPA's runner boundary so conftest JSON stdout is distinct from stderr;
keep stderr available for operational diagnostics. Tests must cover valid stdout
with diagnostic stderr and invalid stdout. Preserve `--no-fail` semantics: valid
results with failing policy checks are evidence, not infrastructure failures.

Preserve PR #233's execution-time input-selection classification when separating
streams: inspect diagnostic stderr for the no-files condition without mixing it
into retained JSON. Keep ordinary execution failures distinct from empty-directory
input. Parse the retained stdout with the existing structured evaluated-input check;
JSON validity alone, including `[]`, `null`, or records without filename/namespace,
does not confirm directory evaluation or permit a synthetic passing assessment.

Retention MUST preserve the existing `opa-input-selection` contracts rather than
replace preflight with a new input-selection mechanism. Prior and current evaluation
outputs, metadata, other providers' artifacts, and partial outputs remain workspace
content excluded from enclosing local directory scans. Explicit workspace targets
and unsafe symlink/hard-link aliases remain rejected. The selected remote clone
remains eligible, but aliases into retained results outside that clone do not.

Keep the existing `opa/repos/<repo>/<branch>` working-clone lifecycle. Do not put
clones under retained results or copy their trees elsewhere. Local file and
directory inputs remain in place, so no symlink snapshot policy is introduced.
Remove the superseded input-hashing code rather than retaining unused hashes or
adding input digests to metadata; retain the original input
path and URL as provenance fields. Persisted conftest filenames can refer to
replaceable/deleted inputs and must not be presented as retained evidence links.

Do not write the legacy Ampel or OPA normalized result summaries in the new layout.
They are documented outputs, so their removal is a breaking change for any external
consumers. The Evaluation Log carries normalized assessments; it does not promise
byte-for-byte equivalence with the legacy summary format.

**Alternatives:** Retaining clones and local snapshots increases storage, adds
copy/symlink rules, and exceeds the agreed output-retention goal. Preserving only
normalized summaries loses raw output detail. A content-addressed input store is
a separate feature, not necessary to preserve evaluation results.

### D4: Evidence identity follows retained artifacts

Use provider/artifact-specific prefixes followed by the evaluation token, for
example `ampel_<token>`, `snappy_<token>`, `arf_<token>`, and `conftest_<token>`.
Use matching source/reference identities such as `openscap-arf_<token>` and
`conftest-result_<token>`. Construct each source/reference pair together from the
same allocated artifact data. Titles and descriptions carry readable target,
branch, spec, or profile context; identity does not require a lossy target encoding.

Include a properly escaped absolute `file://` URL for every reported evidence
artifact, including OPA, identifying the same file as its coordinate per D8. Reuse
the same identity when multiple assessments cite one artifact. Different
evaluations receive different identities even if they evaluated identical inputs.

This deliberately supersedes the old design's cross-run ID stability: an ID is
stable for one stored artifact, not every future evaluation of a logical target.
Keep `SanitizeForID` and `JoinForID` unchanged in this change. #199 can later assess
logical-label collisions without moving evidence or redefining its storage keys.
Do not mark #199 resolved solely because these artifact IDs no longer rely on it.

### D5: Supporting context, not a second evidence authority

Attempt to write `metadata.json` for each completed evaluation, using an explicit
allowlist and this common envelope with provider-specific fields:

```json
{
  "schema_version": 1,
  "evaluation_id": "<evaluation token>",
  "provider": "<provider>",
  "collected_at": "<RFC 3339 collection start>",
  "target": {},
  "configuration": {}
}
```

`evaluation_id` is the token from D2. The Evaluation Log is authoritative for
evidence IDs, coordinates, and digests; metadata is supporting context and MUST NOT
duplicate artifact digests. Metadata does not attest to the identity or integrity
of policy and input bytes, and its presence does not prove publication succeeded.
This existing supporting-metadata scope is retained for human discovery and
correlation, not as a second inventory for future collection. The documented target
and configuration fields below form the allowlist; arbitrary map entries are not
serialized.

Include a readable `target_key` as a correlation aid: `url:` plus a URL's host and
escaped path without scheme, userinfo, query, or fragment for remote targets;
`path:` plus the cleaned, absolute input path for local targets; and `host:` plus
the non-empty TargetID or `system` fallback for OpenSCAP. Use consistent URL and
path formatting across runs. Record the non-secret source URL or input path, target
ID, and applicable branch, scan path, spec or profile separately. A target key alone
does not identify an evaluation: branches, scan paths, specs, and profiles may
distinguish comparisons. Removing URL components and cleaning paths can merge
distinct inputs; keys are grouping hints, not globally unique identities or proof
that targets are identical. Local absolute paths can change after relocation.

Record available configuration context without inventing policy names:

- Ampel: spec reference, resolved spec path, policy path, requirement IDs, and
  content key when resolved through the Generate index.
- OPA: policy directory, namespaces, requirement mapping context, and content key
  when available.
- OpenSCAP: profile, datastream and tailoring paths, and the observed ARF target
  when available, distinct from the request TargetID.

Never serialize target-variable maps or environment variables. Strip URL userinfo,
query, and fragment from metadata, generated labels, titles, and descriptions; do
not record access tokens. Omit unavailable fields. Configuration paths are provenance
hints, not retained snapshots. The existing Generate content key hashes requirement
IDs and a source identifier, not policy bytes. Metadata failures are handled as D6
describes.

**Alternative:** Putting all context in filenames creates unwieldy paths. Extending
provider summaries would maintain parallel normalized formats and require a new
OpenSCAP summary; a small supporting file works across all providers.

### D6: Publish after required outputs are complete

The publication boundary is returning assessments/references in `ScanResponse`.
External tools write directly to fresh evaluation paths, so no copy-after-scan or
rename-over-existing workflow is needed. Wait for tool completion, validate/parse
outputs, close provider-written files, and compute digests before adding the
evaluation's assessments to the response. The required OpenSCAP XCCDF result remains
part of the retained output set, although the ARF is the referenced evidence.

Allocation, required writes/closes, parsing, hashing, and ownership-finalization
failures become operational errors. Preserve these observable channels:

- OpenSCAP: a non-nil RPC error, with no published scan response.
- Ampel: per-evaluation allocation, tool-output, write/close, and parse failures
  become `ScanResponse.Errors`; digest failures retain the existing non-nil RPC
  error behavior. Common workspace/identity setup and ownership-finalization
  failures are RPC errors.
- OPA: per-evaluation allocation, tool-output, write/close, parse, and digest failures
  become `ScanResponse.Errors`. Common workspace/identity setup and
  ownership-finalization failures are RPC errors.

Response error entries identify the failed target/evaluation without secret values.
Exclude its assessments and references while preserving successful evaluations when
the response channel permits continuing. Do not replace errors with empty responses.

OPA input-selection rejection has a wider publication boundary than an ordinary
per-evaluation processing failure. Preserve `inputSelectionError` classification and
the existing preflight/cancellation rejection behavior: if any selected remote branch
fails input-safety preflight or lacks confirmed evaluated directory input, withhold
assessments, evidence entries, and mapping references from every branch of that
configured target, in either branch order. Keep accumulated operational errors and
other configured targets' successful results. Previously written branch artifacts
remain on disk, unreferenced; do not delete them or treat their presence as evidence
publication. Ordinary branch-scoped clone/tool/storage errors remain branch-scoped
unless the existing input-selection contract classifies them otherwise. Stage
branch results until target acceptance rather than publishing references eagerly.

Never reuse a previous result when a tool fails to produce a new file. Attempt to
write metadata after context is finalized; report metadata write or close failures
through the provider's injectable logger with the evaluation token, as warnings
without suppressing otherwise complete evidence. Correlation context may be
unavailable. Any metadata file left behind still undergoes D9 ownership finalization.

An interrupted evaluation can leave unreferenced files. Leave them in place for
diagnosis in this change; do not add cleanup or a new recovery database. The
returned evidence reference and digest are the public contract. No power-loss
durability or tamper-proof storage guarantee is introduced.

### D7: Three-scan example and storage model

The following illustrative tree shows three successive scans. Tokens are shortened
for readability; each completed evaluation contains the required outputs from D1
and best-effort metadata. Interrupted evaluations can contain incomplete outputs.

```text
.complytime/
├── ampel/results/github-com-example-payments/main/
│   ├── branch-protection/
│   │   ├── evaluation-20261102T090000Z-a1/
│   │   ├── evaluation-20261102T100000Z-a2/
│   │   └── evaluation-20261102T110000Z-a3/
│   └── security-settings/
│       ├── evaluation-20261102T090000Z-b1/
│       ├── evaluation-20261102T100000Z-b2/
│       └── evaluation-20261102T110000Z-b3/
├── openscap/results/production-host/stig/
│   ├── evaluation-20261102T090000Z-c1/
│   ├── evaluation-20261102T100000Z-c2/
│   └── evaluation-20261102T110000Z-c3/
└── opa/
    ├── repos/github-com-example-payments/main/  # Current working clone only
    └── results/
        ├── github-com-example-payments/main/
        │   ├── evaluation-20261102T090000Z-d1/
        │   ├── evaluation-20261102T100000Z-d2/
        │   └── evaluation-20261102T110000Z-d3/
        ├── local-service-config/
        │   ├── evaluation-20261102T090000Z-e1/
        │   ├── evaluation-20261102T100000Z-e2/
        │   └── evaluation-20261102T110000Z-e3/
        └── local-kubernetes-manifests/
            ├── evaluation-20261102T090000Z-f1/
            ├── evaluation-20261102T100000Z-f2/
            └── evaluation-20261102T110000Z-f3/
```

Storage grows with output bytes per evaluation, plus metadata and
filesystem overhead. OPA adds raw JSON retention but not historical clone or local
tree storage. Identical outputs are stored separately. Many assessments
can reference one output without creating additional file copies. Failed attempts
can consume space too. No fixed size estimate or storage bound is promised.

### D8: Resolved workspace coordinates and absolute file URLs

The provider-side contract is to resolve the supplied process working directory
once as an absolute workspace root and use it for writes and evidence coordinates.
Providers do not independently interpret `--workspace` or `COMPLYTIME_WORKSPACE`.
They MUST fail the RPC if root resolution fails. Coordinates are relative to that
root, not the Evaluation Log's directory or an auditor's arbitrary current directory.
Process CWD is the workspace handoff mechanism; no additional workspace-context
message, environment override, or duplicate root-selection mechanism is introduced.

This output/coordinate root is distinct from OPA's input-exclusion boundaries.
Preserve PR #233's host-injected `workspace` target variable as a source of the host
`.complytime` exclusion boundary, alongside the configured provider artifact directory
when different. That variable does not select an alternative retained-output root.
Launcher CWD alignment MUST NOT remove the dual-boundary protection.

The external complyctl compatibility change owns workspace selection, with existing
precedence `--workspace`, then `COMPLYTIME_WORKSPACE`, then the invoking directory.
Its proposed launch contract sets each provider subprocess's working directory to
the resolved absolute workspace root, so existing `.complytime`-relative provider
paths use the selected workspace without a new RPC field. Resolve executable and
other caller-relative launch inputs before changing the child working directory;
do not change the parent's process-wide working directory. The inspected complyctl
launcher in `pkg/provider/initialization.go` (also present under
`vendor/github.com/complytime/complyctl/`) currently constructs `exec.Command`
without setting its working directory. A companion compatibility change must
establish this contract and verify Generate and Scan use the same root. This is a
proposed integration adjustment, not a claim about current launcher behavior.

The companion change also owns reliable original-caller identity delivery for
elevated execution (D9). Its implementation mechanism is external follow-up, not an
unanswered provider product decision or an invented new protocol in this proposal.
Task 0.1 establishes the tracked dependency, consumed interface, and compatibility
strategy before implementation starts. Task 5.4 records the actual compatible
revision/version floor and verifies workspace and owner behavior before acceptance
and release.
The companion must land before that verification; this spec-only draft does not
claim it has landed. A provider cannot detect a different workspace selected by a
launcher from CWD alone, so the companion integration tests must verify that handoff.

Before implementation, task 0.1 must select and document an enforceable compatibility
strategy with the companion owners. Candidate mechanisms include dependency or
distribution constraints that pair compatible releases, or a verified launch-contract
signal that lets providers reject incompatible launchers. Documentation of a minimum
version alone informs users but does not enforce the pairing. Any signal requires a
separately agreed interface; do not assume one exists. Provider release acceptance
requires a compatible companion release and the agreed pairing mechanism, with the
actual supported version floor recorded in task 5.4. Neither a valid CWD nor a
successful provider-only test establishes launcher compatibility.

Keep mapping-reference URLs as properly escaped absolute `file://` URLs. Build each
URL from the actual preserved artifact's already-absolute path; reuse `FileURL`
only after validating that input, so its relative-path fallback is not relied upon.
Resolving the coordinate against the selected workspace
and decoding its mapping URL MUST identify the same file. This retains the existing
URL representation rather than introducing scheme-less references for a future
export feature. Validate converted logs against the applicable Gemara schema and
exercise a downstream consumer with the retained URL representation.

The in-place workflow for the evidence owner or another authorized reader is:
establish the selected workspace root, read the log's
evidence mappings, resolve each coordinate against that root or use its absolute
file URL, and verify the file against its recorded digest. This workflow MUST work
without collection, packaging, or a storage integration. Test invocation from
directory A with workspace B, including flag-over-environment precedence, and verify
that provider output is beneath B rather than A. Cover the default workspace and
environment-only selection, URL escaping for spaces, percent signs, and non-ASCII
names, repeated scans, and agreement between coordinates and URLs.

The companion contract must define resolution of caller-relative local inputs and
conftest configuration paths as well as executable paths before changing child CWD.
Verify configuration discovery, explicit relative configuration, ignore precedence,
and relative input selection in the new evaluation context. Preserve PR #233's
fail-closed behavior when an applicable ignore rule cannot be determined.

Workspace-internal metadata paths use the same root-relative convention. External
paths, such as a system datastream, remain absolute provenance hints. Absolute file
URLs describe the originating filesystem and are not promised to survive workspace
relocation. Coordinates can be resolved against a copied workspace with its layout
preserved, but that does not relocate the original absolute URLs. A future optional
complyctl packaging feature must map moved artifacts without changing the log; this
change neither implements that mechanism nor depends on it.

**Alternative launch mechanism:** Exporting the workspace only through an
environment variable would require changing every existing `.complytime`-relative
provider path. Child CWD preserves that convention and is the sole provider-side
workspace input. No extra workspace-confirmation protocol is needed for that choice.

**Alternative:** Scheme-less relative URLs would require establishing Gemara schema
and consumer support and an external resolution base. Export portability alone is
not a reason to change the runtime URL representation. Copying an entire workspace
is not a substitute for an explicitly designed future evidence package.

### D9: Finalize evidence for the original invoking user

The author-confirmed access policy is 0700 evaluation directories and 0600 files,
owned by the original invoking user's numeric UID and GID,
not necessarily the effective account running an elevated scan. Normal invocation
uses the invoking account. Elevated execution on a user's behalf uses that original
user; direct root invocation with no originating user remains UID/GID 0. Complyctl's
companion launch contract owns reliable identity establishment and delivery for
elevated execution. Non-elevated execution uses the operating system's invoking
UID/GID without requiring a new caller-identity message.
Providers MUST NOT infer an owner from arbitrary `SUDO_UID`, `SUDO_GID`, `USER`,
target variables, or workspace metadata. Missing, malformed, or unverified identity
for an elevated-on-behalf invocation is an RPC error, not permission to guess an
owner or silently keep root. Direct-root identity must be established as such,
not inferred as a fallback when original-user identification fails.

Before implementation, the companion contract MUST specify how identity provenance
is established, how direct-root and elevated-on-behalf invocation are distinguished,
and how the provider validates the delivered context. Numeric UID/GID values alone
do not prove caller identity. The launch-context adapter consumes that agreed
interface and supplies validated ownership context to the shared allocator; the
allocator does not infer trust from transport fields or arbitrary environment values.

During privileged execution, retain control of the fresh evaluation directory until
all tool output is complete, required files are closed, validated and hashed, and
best-effort metadata has been attempted. Finalize ownership through constrained
handles, then verify the required files and directory have the selected UID/GID
and 0700/0600 modes before returning assessments/references. Hand off the evaluation
directory last, after its files; tools must have stopped writing before handoff.
Normal non-root execution verifies ownership without requiring privileged chown.
Failed ownership finalization returns an RPC error and publishes no response.

Changes are limited to the newly allocated evaluation directory and its contents.
Never recursively chown an existing workspace, shared context parent, legacy file,
clone, input, or earlier evaluation. Resolve any required new parent-directory
access in the companion workspace-setup contract; verify the intended owner can
traverse the full path. If existing parents prevent access, fail with an actionable
error rather than changing unrelated ownership or publishing inaccessible evidence.
Caller ownership does not grant other auditors access or promise immutable storage.

Use injected identity/filesystem operations to exercise normal, elevated-on-behalf,
direct-root, invalid identity, symlink substitution, and handoff/traversal failure
paths in unprivileged unit tests. A controlled privileged integration test must
also verify real UID/GID results and reading/hashing as the original non-root user;
record that prerequisite when it cannot run locally. No implicit ACL, shared-group,
or automatic privilege escalation mechanism is introduced.

## Risks / Trade-offs

- **Result retention is not input reproducibility** → State this in README and
  man pages. OPA output preserves reported findings, not historical input bytes
  or policy bundles. This is the agreed change from input-path evidence.
- **Storage grows until explicitly managed** → Document this behavior; defer
  cleanup until after this feature merges. Never delete retained evidence during
  normal scans in this change.
- **Retained history increases OPA preflight work** → Workspace identity collection
  traverses retained history to detect aliases, so its cost grows with accumulated
  files and directories, not just current target size. Measure representative
  histories using the existing preflight benchmark approach; do not promise bounded
  scan overhead or skip historical identities to improve timings. Cover existing
  private evaluation directories: inaccessible history can fail preflight closed.
  Report actionable access errors without widening permissions or changing old owners.
- **Readable grouping can collide** → Allocate separate unique leaves and attempt
  to retain context in metadata. Metadata keys can also collide after normalization
  and are only correlation hints. #199 remains independent of storage correctness.
- **Existing consumers expect fixed paths or IDs** → Mark the path and OPA
  evidence semantics, including workspace-relative coordinates, as breaking changes;
  direct consumers to returned coordinates and the explicit workspace base. Absolute
  file URLs retain their representation but identify the newly allocated artifacts.
- **Global mutable OpenSCAP paths** → Pass evaluation paths explicitly through
  scan, parser, hash, and mapping functions rather than changing package globals
  around a scan, which would introduce a race.
- **OPA clone/config concurrency is already limited** → Test concurrent artifact
  allocation independently; do not claim serialized input acquisition. No new
  concurrent execution is introduced by this change.
- **Raw outputs may contain sensitive findings** → Keep private evaluation
  directories and files at the author-selected 0700/0600 modes, and
  finalize ownership for the invoking user per D9. Metadata excludes credentials;
  raw evidence is not rewritten or redacted because that would change its bytes.
- **Workspace relocation and manual edits** → Absolute file URLs remain specific to
  the originating filesystem. Relative coordinates need an explicit workspace root;
  future packaging must supply relocation mapping without rewriting the log. Digests
  detect artifact changes, but do not authenticate the log or create immutable archives.
- **Metadata failure** → Warn and publish complete evidence; contextual correlation
  may be unavailable, but the Evaluation Log still identifies and verifies it.

## Decision record and external follow-up

The author has resolved the evidence-access and runtime-reference product choices:

- **Workspace:** use the provider process CWD supplied by complyctl as the workspace
  root, with coordinates relative to it. Do not add a second root-selection protocol.
- **URLs:** retain absolute `file://` mapping URLs identifying the same files.
- **Ownership:** finalize files for the original invoking user; elevated-on-behalf
  scans do not require that user to read evidence as root. Direct root stays root.
- **Modes:** use 0700 evaluation directories and 0600 files, including supporting
  metadata and tool-written outputs. The author explicitly confirmed these modes.
- **Workflow direction:** retain these OpenSpec artifacts; the author identified
  the Speckit instruction as outdated and explicitly declined migration.

Complyctl workspace launch alignment and reliable elevated-caller identity delivery
are separately owned prerequisites. Task 0.1 must record the companion issue/spec,
consumed interface, identity trust model, compatibility strategy, and merge sequencing
before implementation. No matching companion reference is established by this draft;
the dependency remains blocking rather than being treated as an integration-only
detail. Task 5.4 records the actual compatible revision/version floor and verifies the
landed implementation before acceptance and release. These are not new product
choices awaiting this author's answer. Schema validation, actual ownership handoff,
and end-to-end consumer checks remain tests to perform, not claims that future
behavior is already implemented.
This decision record does not constitute council, implementation, or merge approval.

### Companion contract acceptance record

The companion launcher contract is proposed, not agreed or available in a supported
release. Before task 0.1 can complete, record the following evidence here and link
the accepted companion issue/spec. An issue describing the desired behavior alone
does not establish the consumed interface or satisfy this prerequisite.

| Contract element | Required acceptance evidence | Current state |
| --- | --- | --- |
| Tracking and ownership | Companion complyctl issue/spec, responsible owner, and accepted contract revision | No companion reference or acceptance is recorded |
| Workspace handoff | Child CWD equals the resolved absolute workspace for Generate and Scan; document symlink resolution and caller-relative executable/input/configuration handling | D8 defines the required behavior; launcher agreement is outstanding |
| Identity delivery | Name the consumed interface, its producer, identity provenance and validation, and how direct-root invocation differs from elevation on behalf of a user | D9 defines the required behavior; transport and trust agreement are outstanding |
| Provider consumption | Provider adapters resolve roots and validate launch context; shared storage receives explicit roots and verified ownership without importing provider packages | Defined in D1; the identity adapter awaits the companion interface |
| Compatibility | Define how unsupported launchers are excluded, how users identify a compatible build, and what dependency/release mechanism enforces the pairing | No compatibility mechanism or version floor is established |
| Merge and release order | Agree the companion/provider merge order and when the compatible launcher must be available; link actual landed revisions before release acceptance | Companion landing precedes task 5.4 verification; the detailed sequence awaits agreement |
| Verification | Link default/environment/flag workspace tests, caller-relative input/configuration tests, identity rejection tests, and a real privileged ownership-handoff result | Future verification under task 5.4; no passing result is claimed |

Do not infer launcher compatibility from a valid process CWD: the provider cannot
detect a workspace selected elsewhere by an older launcher. Do not infer identity
trust from numeric UID/GID values or an environment variable name. The accepted
contract must explain these guarantees rather than rely on provider guesses.
A released version need not exist during design review; task 5.4 records the actual
landed revision and supported version floor after the compatible implementation
exists. Keep collection, packaging, storage, and retention-policy work outside this
dependency.

## Repository instruction conflicts

The selected design and author direction are explicit. The following loaded rules
still conflict with them and are reported for governance handling, not re-presented
as unanswered evidence-design questions. Recording the decisions does not change
these rules, their severity, or review requirements.

- **Permission-mode instruction conflict (HIGH):** The selected 0700/0600 modes
  conflict with `.opencode/uf/packs/go.md` SC-005's 0755/0644 rule. Caller ownership
  does not waive that rule. Obtain reconciliation through the applicable governance
  process before implementing modes. Do not edit convention packs or
  make findings world-readable to clear review in this change.
- **Workflow instruction conflict (HIGH):** The user has identified the Speckit
  tier instruction as outdated and requested no migration. The loaded AGENTS.md
  still requires Speckit for cross-repository work. These provider artifacts remain
  in OpenSpec, with complyctl changes externally owned; no governance rule is changed
  or treated as waived. Maintainer reconciliation of the conflicting instructions
  remains outstanding rather than being presented as implementation approval.

For the permission conflict, the readiness record MUST identify the authorized
maintainer decision, its scope, the decision venue and link, and the resulting
applicable instruction. Until that evidence is recorded, reconciliation remains
outstanding. This record does not authorize an agent to amend or waive a convention
pack, and it does not substitute broader access for the selected private modes.

## Migration Plan

1. Review and approve these artifacts; commit and push specifications before
   implementation begins, following repository workflow.
   Establish task 0.1's companion contract and reconcile the reported permission-mode
   instruction conflict before affected implementation begins. Do not make findings
   world-readable or modify governance rules as part of retention implementation.
2. Implement allocation and metadata helpers, then integrate each provider and
   update evidence-source tests. Keep the plugin protocol unchanged.
3. Update man-page sources and generated pages, README guidance, and release notes
   where maintained. Assess AGENTS structural changes and file the required website
   issue before the implementation PR merges.
4. Run three-scan preservation tests for every provider and all OPA input types;
   verify returned coordinates, stored bytes, digests, reference URLs, and IDs.
5. Deploy with new paths only. Leave legacy files and old logs untouched; there is
   no reliable recovery of evidence already overwritten before upgrade.

Rollback to an older binary leaves new evaluation directories readable but resumes
the old overwrite behavior for subsequent scans. Existing logs are not migrated in
either direction. Full implementation validation and review requirements are listed
in [tasks.md](tasks.md).
