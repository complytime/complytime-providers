## Context

The export infrastructure was added to complytime-providers via the `add-export-rpc` OpenSpec change. It implemented the `provider.Exporter` interface from complyctl's SDK for the OpenSCAP and AMPEL providers, enabling OTLP evidence emission via ProofWatch.

The upstream complyctl repository has now removed this entire subsystem (PR [#617](https://github.com/complytime/complyctl/pull/617), issue [#606](https://github.com/complytime/complyctl/issues/606)). The export feature was speculative infrastructure built before the backend design was finalized. Export will be redesigned when the backend shape is known.

The providers currently depend on `complyctl v1.0.0-beta.0`, which still has the export types. The next complyctl version will not. This change must land before bumping the complyctl dependency.

## Goals / Non-Goals

**Goals:**
- Remove all export code from OpenSCAP and AMPEL providers so they compile against the post-removal complyctl SDK
- Remove export-only dependencies to reduce binary size and attack surface
- Update documentation to reflect the current provider capabilities
- Maintain a clean OpenSpec history by marking superseded artifacts

**Non-Goals:**
- Redesigning or replacing the export mechanism (that is a future upstream concern)
- Bumping the complyctl dependency (separate follow-up change)
- Modifying the OPA provider (it never implemented Export)
- Removing the `add-export-rpc` OpenSpec artifacts from git history (they remain as historical record; status is updated to SUPERSEDED)

## Decisions

### 1. Remove export code before bumping complyctl dependency

**Decision**: Land this removal as a standalone change on the current `complyctl v1.0.0-beta.0` dependency, then bump complyctl in a separate follow-up.

**Rationale**: This keeps the change single-concern (Constitution III). The removal can be verified against the current SDK -- all existing tests must continue to pass. The complyctl bump is a separate atomic change with its own potential breakage surface.

**Alternative considered**: Combine removal + dependency bump in one PR. Rejected because it mixes two concerns and makes bisecting harder if something breaks.

### 2. Delete entire export packages rather than stubbing

**Decision**: Delete the `export/` packages entirely rather than leaving empty stubs or no-op implementations.

**Rationale**: The upstream interface no longer exists -- there is nothing to stub against. Dead code violates the zero-waste behavioral rule. The packages have no consumers other than the `Export()` methods being removed.

### 3. Mark add-export-rpc as SUPERSEDED rather than deleting

**Decision**: Update `.openspec.yaml` status to `superseded` rather than removing the directory from the repository.

**Rationale**: Consistent with how complyctl handled its own spec artifacts. The files serve as historical record of why the feature existed and how it was designed, which will be valuable context when export is eventually redesigned.

## Risks / Trade-offs

**[Risk] Missed export reference causes compile failure** -- Mitigated by running `go build ./...` and `go test ./...` as verification steps. The compiler will catch any remaining references to deleted types.

**[Risk] go mod tidy removes a dependency still needed transitively** -- Mitigated by `go mod tidy` only removing packages with zero import paths remaining. Verified by full build + test after tidy.

**[Risk] Documentation references to export persist in unexpected locations** -- Mitigated by grep-based search for export-related terms across all markdown files before marking complete.
