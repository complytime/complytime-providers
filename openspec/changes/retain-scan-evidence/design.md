# Design

## Context

See [proposal.md](proposal.md) for motivation and scope. The existing implementation
already carries evidence fields through the complyctl SDK into the Evaluation Log;
this change does not need an RPC extension.

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

**Non-Goals:**

- General logical-ID encoding repair (#199), input snapshots, policy archives,
  directory hashing, filesystem deduplication, or an evidence database.
- Reworking OpenSCAP target dispatch or mutable Generate-time configuration.
- Claiming full concurrent-scan safety for OPA's shared clone/policy workspaces.
  Artifact allocation is concurrency-safe; input acquisition races remain a
  separate concern and must not be disguised by the retention documentation.
- Implementing cleanup, including a new post-scan clone deletion lifecycle.

## Decisions

### D1: Group by readable context; allocate one directory per evaluation

Use the following provider-relative patterns. Each label is one filesystem
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

Use root-constrained filesystem operations for allocation and provider-owned
writes; reject symlinked context parents instead of following them outside the
results root. Use private evaluation directories (0700) and provider-written
files (0600). Tool-written files are protected by the private evaluation directory.
Centralize modes, label limits, time format, and artifact basenames.

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
existing-name collision with a fresh token using a bounded shared retry limit;
return an operational error on exhaustion or any other allocation failure. Do not
use an existence-check-then-create sequence. Pass an evaluation value carrying
time, token, and allocated directory to provider-specific operations; avoid globals.

The artifact-instance identity includes provider, token, and artifact kind, so
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

Include a properly escaped workspace-relative URL reference for every retained
evidence artifact, including OPA, with the base defined in D8. Reuse the same
identity when multiple assessments cite one artifact. Different
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

Allocation, required writes/closes, parsing, and hashing failures become operational
errors. Multi-evaluation providers can continue their existing per-target error
behavior, but must exclude the failed evaluation's assessments and references.
A provider-level error can still fail the RPC where that is the established path.
Never reuse a previous result when a tool fails to produce a new file. Attempt to
write metadata after context is finalized; report metadata write or close failures
as warnings without suppressing otherwise complete evidence. Correlation context
may be unavailable for such an evaluation.

An interrupted evaluation can leave unreferenced files. Leave them in place for
diagnosis in this change; do not add cleanup or a new recovery database. The
returned evidence reference and digest are the public contract. No power-loss
durability or tamper-proof storage guarantee is introduced.

### D7: Three-scan example and storage model

The following illustrative tree shows three successive scans. Tokens are shortened
for readability; every evaluation directory contains the output set from D1.

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

### D8: Workspace-relative evidence references

Express coordinates as paths relative to the workspace root containing
`.complytime`. Express mapping-reference URLs as escaped relative URI references
to the same artifacts, without a `file:` scheme. Interpret both against that
workspace root, not against the Evaluation Log's directory or the auditor's
arbitrary current directory. Document this base and how to locate the workspace.
For example, a log in `.complytime/scan/` does not by itself define the URI base;
generic document-relative URL resolution will not work without the workspace base.

Transfer the Evaluation Log with the referenced workspace tree, retaining their
relative layout; moving the log alone is insufficient. Workspace-internal metadata
paths follow the same root-relative convention. External paths, such as a system
datastream, remain absolute provenance hints and need not survive relocation.
Test URI escaping for spaces, percent signs, and non-ASCII names and verify both
references resolve to the same file using the explicit workspace base.

**Alternative:** Absolute file URLs resolve on the original host but embed its
filesystem layout and do not follow a workspace moved to another host. Keeping
absolute URLs alongside relative coordinates leaves two incompatible bases.

## Risks / Trade-offs

- **Result retention is not input reproducibility** → State this in README and
  man pages. OPA output preserves reported findings, not historical input bytes
  or policy bundles. This is the agreed change from input-path evidence.
- **Storage grows until explicitly managed** → Document this behavior; defer
  cleanup until after this feature merges. Never delete retained evidence during
  normal scans in this change.
- **Readable grouping can collide** → Allocate separate unique leaves and attempt
  to retain context in metadata. Metadata keys can also collide after normalization
  and are only correlation hints. #199 remains independent of storage correctness.
- **Existing consumers expect fixed paths or IDs** → Mark the path and OPA
  evidence semantics, including workspace-relative coordinates and scheme-less
  reference URLs, as breaking changes; direct consumers to returned coordinates
  and the explicit workspace base.
- **Global mutable OpenSCAP paths** → Pass evaluation paths explicitly through
  scan, parser, hash, and mapping functions rather than changing package globals
  around a scan, which would introduce a race.
- **OPA clone/config concurrency is already limited** → Test concurrent artifact
  allocation independently; do not claim serialized input acquisition. No new
  concurrent execution is introduced by this change.
- **Raw outputs may contain sensitive findings** → Keep private evaluation
  directories and existing access expectations. Metadata excludes credentials;
  raw evidence is not rewritten or redacted because that would change its bytes.
- **Workspace relocation and manual edits** → Preserve the workspace layout and
  resolve relative references against its root. Digests detect artifact changes,
  but this change does not create immutable archives or authenticate the log.
- **Metadata failure** → Warn and publish complete evidence; contextual correlation
  may be unavailable, but the Evaluation Log still identifies and verifies it.

## Open review decisions

This proposal remains a draft. Review identified the following decisions for
maintainers; the text above records the proposed design, not settled approval.

- **Workspace base (HIGH):** Define how providers determine the coordinate base
  when complyctl's `--workspace` or `COMPLYTIME_WORKSPACE` selects a directory other
  than the plugin process's current directory. Confirm the launcher/runtime
  contract and add a non-default-workspace acceptance case before implementation;
  documentation of the intended base alone does not align differing write roots.
- **Elevated OpenSCAP access (HIGH):** Decide whether private evidence from `sudo`
  scans requires elevated reading/verification, or needs a supported shared-access
  model. Reconcile D1's proposed 0700/0600 modes with the invoking auditor's access,
  document ownership and metadata readability, and verify the chosen workflow.
- **Workflow tier (HIGH):** Decide whether required companion complyctl work makes
  this a strategic cross-repository change requiring Speckit, or should become an
  independently scoped follow-up with explicit sequencing. The current tactical
  classification and task 5.4 coupling require reconciliation before implementation.
- **Downstream relative-URL compatibility (HIGH):** Establish whether the Gemara /
  Evaluation Log schema permits scheme-less relative `MappingReference.url` values.
  Verify a converted log against that schema and a downstream consumer; preserving
  strings through serialization alone does not establish schema validity or usable
  resolution. If unsupported, revisit D8 with maintainers rather than silently
  shipping an incompatible reference representation.

## Migration Plan

1. Review and approve these artifacts; commit and push specifications before
   implementation begins, following repository workflow.
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
