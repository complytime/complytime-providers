# Proposal

## Why

Both the OPA and Ampel providers write their `Generate`-phase artifacts
(`scan-config.json`, merged policy bundle) to a single shared path per
provider workspace. `complyctl` calls `Generate` once per
`(evaluator, target)` pair, so when a policy references multiple targets
that resolve to different requirement sets or different OPA bundles, each
`Generate` call silently overwrites the previous call's artifacts. `Scan`
then reads the last-written file and applies it to every target, producing
incorrect compliance results with no error or warning. This is a
data-correctness bug in a compliance tool (GitHub issue #190).

## What Changes

- Introduce a content-addressed directory scheme for `Generate`-phase
  artifacts in both the OPA and Ampel providers. Instead of one shared
  `scan-config.json` (and merged policy bundle for Ampel), each unique
  set of matched requirements writes to its own content-keyed
  subdirectory.
- Introduce a `generate-index.json` file that maps target identifiers to
  their content-key directory, enabling `Scan` to locate the correct
  artifacts per target without requiring information absent from
  `ScanRequest`.
- Move `Scan`'s scan-config read from a single pre-loop read into a
  per-target read, using the index to resolve the correct content
  directory for each target.
- Applies uniformly to OPA (`opa_bundle_ref` and `ComplypackContentPath`
  paths) and Ampel providers.

## Capabilities

### New Capabilities

- `target-scoped-generation`: Per-target isolation of `Generate`-phase
  artifacts (scan-config, policy bundles) so that multi-target policies
  produce correct, independent scan results per target.

### Modified Capabilities

<!-- No existing spec-level behavior changes. -->

## Impact

- **OPA provider**: `cmd/opa-provider/config/config.go` (new path
  helpers), `cmd/opa-provider/generate/scanconfig.go` (content-key
  computation, index read/write), `cmd/opa-provider/server/server.go`
  (Generate write path, Scan read path restructured per target).
- **Ampel provider**: `cmd/ampel-provider/config/config.go` (new path
  helpers), `cmd/ampel-provider/generate/scanconfig.go` (content-key
  computation, index read/write), `cmd/ampel-provider/server/server.go`
  (Generate write path, Scan read path restructured per target,
  `PolicyPath` resolved per target).
- **Backward compatibility**: Single-target policies continue to work
  identically. The `generated/` directory (OPA) becomes vestigial for the
  `opa_bundle_ref` path but remains for the index file. No changes to
  the `complyctl` plugin interface (`GenerateRequest`/`ScanRequest`).
- **Test impact**: Existing single-target tests pass unchanged. New
  multi-target integration tests required for both providers.
