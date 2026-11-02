# Proposal

Status: Draft for design feedback, not approved for implementation. The final
review identified unresolved workspace-base, elevated-scan access, workflow-tier,
and downstream relative-URL compatibility decisions, recorded under
`Open review decisions` in the design.

## Why

Provider scans overwrite artifacts referenced by earlier Evaluation Logs, breaking
the link between an assessment, its evidence coordinate, and its digest.
[Issue #198](https://github.com/complytime/complytime-providers/issues/198)
describes the need for retained scan evidence across supported providers and OPA
target types. Auditors start with the Evaluation Log, follow its evidence mapping,
and verify the retained raw output against the logged digest. Retention preserves
reported results, not a guarantee of historical re-evaluation or detection of every
target change.

## What Changes

- Retain each evaluation's outputs in a new, exclusively allocated directory with
  a UTC timestamp and unique suffix, beneath readable target and evaluation-context
  directories. Keep repository, branch, collection spec, or OpenSCAP profile visible
  where available.
- Retain Ampel's snappy and ampel attestations, and
  OpenSCAP's ARF and XCCDF results. Allocate per target/branch/spec for Ampel, per
  target/branch or local target for OPA, and per executed system scan for OpenSCAP.
- **BREAKING**: OPA evidence references retained raw conftest JSON output instead
  of the evaluated input path. Apply this uniformly to remote repositories, local
  files, and local directories, and compute a file digest in every case.
- Keep OPA clones as replaceable working data. Do not retain historical clones,
  snapshot local inputs, or introduce a second clone/copy for evidence retention.
- Carry allocated artifact paths through parsing, hashing, and response construction.
  Qualify evidence and mapping-reference IDs by evaluation instance so references
  identify the retained artifact rather than a moving target.
- **BREAKING**: Scan output paths and evidence IDs change, including the token-qualified
  prefix and separator formats specified in the evidence-source delta. Fixed-path aliases are
  not written; consumers must use the returned evidence coordinate. Existing
  artifacts remain untouched and historical logs are not rewritten.
- **BREAKING**: Stop writing the legacy Ampel and OPA normalized result summaries.
  The Evaluation Log carries normalized assessments; the raw retained output is the
  evidence. Existing summary files remain untouched.
- **BREAKING**: Absolute `file://` mapping-reference URLs become escaped, scheme-less,
  workspace-root-relative URI references; evidence coordinates become paths relative
  to that same explicitly documented workspace base. Keep the Evaluation Log and workspace
  together when transferring evidence.
- Attempt to record non-secret target and configuration context in a small
  per-evaluation metadata file. It supports discovery and correlation but is not
  the authority for evidence identity or digests, and a write failure is a warning.
- Document storage growth and the distinction between retained evaluation output
  and reproducible historical inputs. Update provider man pages and user guidance.

Automatic cleanup, retention configuration, compression, content deduplication,
Generate-time policy retention, and input snapshots are outside this change.
[Issue #199](https://github.com/complytime/complytime-providers/issues/199) remains
a separate assessment of logical identity encoding; storage and artifact-instance
identity must not depend on that lossy encoding being unique.

## Capabilities

### New Capabilities

- `scan-artifact-retention`: Readable per-evaluation storage, exclusive allocation,
  preserved outputs, context metadata, and operational failure behavior.

### Modified Capabilities

- `evidence-source`: Retained artifact coordinates and digests, OPA result-file
  evidence for all target types, instance-qualified references, consistent collection
  timestamps, and documentation of the evidence lifecycle.

## Impact

- Shared allocation and metadata helpers under `internal/evidence/`; provider
  config, scan, results, and server packages; OPA command-output handling.
- OPA's loader keeps its existing replace-before-clone lifecycle. Historical
  input availability and deterministic re-evaluation are not guarantees of this
  feature; persisted conftest filenames are provenance, not retained input links.
- No complyctl RPC/protobuf changes, new flags, or new external dependencies are
  required. The existing Evaluation Log conversion preserves source fields.
- Documentation implementation includes `docs/man/*.md` and generated man pages,
  README guidance, and assessment of `CHANGELOG.md` and `AGENTS.md` impact.
- Coordinate with complyctl to update cross-repository integration assertions and
  evidence documentation for the new paths and workspace-relative URL semantics.
- This is a single-repository tactical change with two user stories: preserve
  evaluation evidence, and locate/verify it using readable context and log links.
- This commit contains specification artifacts for review. Implementation follows
  approval and the repository's spec commit/push prerequisite. A website issue is
  required before the eventual user-facing implementation PR merges.
