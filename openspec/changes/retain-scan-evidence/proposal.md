# Proposal

Status: Implementation proposal awaiting approval. The selected contract uses
complyctl's existing workspace variables, workspace-relative inputs/configuration,
absolute file URLs, and execution-account ownership with private 0700/0600 modes.
No complyctl changes are required. The permission-rule conflict remains reported
in the design; no governance rule is changed or waived here.

## Why

Provider scans overwrite artifacts referenced by earlier Evaluation Logs, breaking
the link between an assessment, its evidence coordinate, and its digest.
[Issue #198](https://github.com/complytime/complytime-providers/issues/198)
describes the need for retained scan evidence across supported providers and OPA
target types. Auditors start with the Gemara Evaluation Log, follow its evidence
mapping, and verify the preserved raw output against the logged digest. The log
defines the evidence set; providers decide which files to report. Unreferenced
supporting outputs are not implicitly evidence. Here, retention means local
preservation across scans, not a managed long-term storage lifecycle, historical
re-evaluation, or detection of every target change.

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
- Preserve the input-isolation contracts merged in PR #233: accumulated retained
  outputs never become target input, and input-selection rejection of one remote
  branch withholds assessments and evidence references from its configured target.
  Already written artifacts remain unreferenced rather than being deleted.
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
- **BREAKING**: Evidence coordinates explicitly use the resolved workspace root as
  their base, replacing the implicit process-CWD interpretation. Providers write
  beneath that same root, including when an explicit
  workspace differs from the invoking directory. Keep properly escaped absolute
  `file://` mapping-reference URLs identifying the same preserved files. Evidence is
  locatable and digest-verifiable in the originating workspace without packaging.
- **BREAKING**: Resolve relative provider input/configuration paths against the
  selected workspace, not the invoking directory. Absolute paths remain absolute.
  Tool configuration discovery uses the workspace without process-wide chdir.
- Keep new evaluation directories and files owned by the provider's execution
  account. Normal user scans are user-owned; root-run scans, including sudo, remain
  root-owned and require authorized elevated access for other readers. Automatic
  original-user ownership handoff is deferred.
- Use private 0700 evaluation directories and 0600 evidence/supporting files,
  including tool-written outputs and metadata. Execution-account ownership does not
  grant the original non-root user access after a root-run scan.
- Attempt to record non-secret target and configuration context in a small
  per-evaluation metadata file. It supports discovery and correlation but is not
  the authority for evidence identity or digests, and a write failure is a warning.
- Document storage growth and the distinction between retained evaluation output
  and reproducible historical inputs. Update provider man pages and user guidance.

Automatic cleanup, retention configuration, compression, content deduplication,
Generate-time policy retention, and input snapshots are outside this change.
Collection, validation of a collected evidence set, and optional packaging are a
separate future complyctl core capability, enabled by this mapping contract rather
than required for it. That capability will use the Gemara Evaluation Log as its
source of truth and must map relocated artifacts without modifying the log; no
packaging format or relocation mechanism is designed here. Storage backends,
transfers, access policies, archival, and long-term retention remain customer-owned.
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
- Provider configuration supplies resolved workspace/results roots and OS-derived
  execution identity. Shared helpers validate confinement, allocate artifacts, and
  verify ownership/modes without importing provider command packages, selecting
  workspace precedence, or interpreting caller identity transport.
- OPA's loader keeps its existing replace-before-clone lifecycle. Historical
  input availability and deterministic re-evaluation are not guarantees of this
  feature; persisted conftest filenames are provenance, not retained input links.
- The existing Evaluation Log conversion preserves the required evidence fields.
  No evidence-mapping RPC extension, new user flags, or new dependencies are
  proposed. Existing Generate global variables and Scan target variables supply
  `workspace`; no launcher CWD change, caller-identity interface, new handshake, or
  paired complyctl release is required.
- Documentation implementation includes `docs/man/*.md` and generated man pages,
  README guidance, and assessment of `CHANGELOG.md` and `AGENTS.md` impact.
- Verify integration with unchanged complyctl, recording the tested revision and
  its existing workspace-variable delivery and evidence conversion. Integration
  verification does not require companion implementation or release sequencing.
- These OpenSpec artifacts cover provider changes with two user stories: preserve
  evaluation evidence, and locate/verify it using readable context and log links.
  Implementation is confined to this repository; complyctl is an unchanged
  integration consumer. The single-repository scope retains OpenSpec without
  changing the workflow-tier rules.
- This commit contains specification artifacts for review. Implementation follows
  approval and the repository's spec commit/push prerequisite. A website issue is
  required before the eventual user-facing implementation PR merges.
