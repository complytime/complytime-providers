## Context

The ampel provider implements the `complyctl` gRPC plugin interface with `Generate` and `Scan` RPCs. During `Generate`, the provider matches requested requirement IDs against granular AMPEL policies and writes a merged policy bundle. During `Scan`, the provider runs the AMPEL toolchain and converts findings into `AssessmentLog` entries via `ToScanResponse`.

Currently, `ToScanResponse` only creates assessments from findings. Requirements where all checks pass produce no findings and are silently omitted from the response. The OPA provider solved an identical issue by persisting a reverse mapping of requirement IDs during `Generate` and using it during `Scan` to synthesize passing assessments.

The ampel case is simpler than OPA: the AMPEL policy ID **is** the Gemara requirement ID (no Rego namespace translation needed), so no reverse mapping is required — just the list of matched requirement IDs.

## Goals / Non-Goals

**Goals:**

- Every requirement ID matched during `Generate` SHALL appear in the `ScanResponse` with at least one `Step`.
- Requirements with zero findings SHALL produce a passing `AssessmentLog` with synthetic steps identifying each scanned repository/branch.
- The implementation SHALL follow the scan-config persistence pattern established by the OPA provider for consistency across providers.

**Non-Goals:**

- Fixing `steps: []` in the final evaluation log YAML (complyctl framework issue, tracked in complyctl#573).
- Fixing empty `target` in the evaluation log (complyctl framework issue, tracked in complyctl#574).
- Changing the AMPEL toolchain or policy format.
- Refactoring the OPA provider's scan config or results code.

## Decisions

### 1. Persist requirement IDs via scan-config.json

**Decision:** Add `WriteScanConfig` and `ReadScanConfig` helpers under `cmd/ampel-provider/generate/` (new package), writing a `scan-config.json` file to the generated directory during `Generate`.

**Rationale:** This mirrors the OPA provider's established pattern. The ampel provider's config package handles directory paths, not state persistence. A dedicated `generate` package keeps the scan config logic colocated with the Generate flow, consistent with OPA's `cmd/opa-provider/generate/scanconfig.go`.

**Alternative considered:** Storing requirement IDs in the existing config package or embedding them in the AMPEL policy bundle. Rejected because: the config package is purely path resolution, and embedding in the policy bundle would couple provider metadata with AMPEL tool input.

### 2. ScanConfig structure: requirement IDs only (no reverse mapping)

**Decision:** The ampel `ScanConfig` stores `RequirementIDs []string` (the Gemara requirement IDs) and `GeneratedAt string`. No reverse mapping field.

**Rationale:** Unlike the OPA provider, the ampel provider does not translate between tool-specific identifiers and Gemara requirement IDs. The AMPEL policy ID is the requirement ID, and `TenetID` in findings is derived from the policy ID with a `check-` prefix. A simple list is sufficient.

### 3. Extend ToScanResponse signature to accept all requirement IDs

**Decision:** Change `ToScanResponse(repoResults []*PerRepoResult)` to `ToScanResponse(repoResults []*PerRepoResult, allRequirementIDs []string)`.

**Rationale:** This is the minimal change to make the full set of evaluated requirements visible to the assessment-building logic. When `allRequirementIDs` is nil (e.g., scan config missing), the function falls back to current behavior (findings-only) without error, maintaining backward compatibility.

### 4. Synthetic steps mirror repository scan context

**Decision:** Synthetic passing steps SHALL use the same `repoName@branch` naming convention as finding-derived steps and SHALL be built from the `repoResults` list (one step per non-error repository result).

**Rationale:** This matches the OPA provider's `buildSyntheticSteps` pattern and ensures that the synthesized assessment steps reflect the actual scan targets, providing meaningful context in the evaluation log.

## Risks / Trade-offs

- **[Risk] Generate not run before Scan** → The `Scan` method will log a warning if `scan-config.json` is missing and fall back to findings-only behavior (no synthetic assessments). This matches the OPA provider's degradation strategy.
- **[Risk] Requirement ID list stale after policy update** → Users must re-run `Generate` after policy changes. This is the existing contract for all providers and is enforced by complyctl's workflow.
- **[Trade-off] New package vs. extending existing** → Adding `cmd/ampel-provider/generate/` introduces a new package. The benefit is consistency with OPA and separation of concerns. The cost is one more package in the provider tree.
