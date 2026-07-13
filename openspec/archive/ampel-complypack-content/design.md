## Context

The ampel provider's `Generate()` method currently resolves granular policy files from two sources:
1. Default path: `config.GranularPolicyDirPath()` (hardcoded to `<workspace>/ampel/granular-policies/`)
2. Override: `req.GlobalVariables["ampel_policy_dir"]` (custom directory)

The OPA provider already supports a third, higher-priority source via `req.ComplypackContentPath` (added in complyctl#536). This field provides a filesystem path to either a directory or a `content.tar.gz` archive containing policy content distributed through complypacks. The ampel provider needs the same capability.

The OPA provider's implementation in `cmd/opa-provider/server/server.go` (lines 180-316) provides the reference pattern: `resolvePolicyDir()` -> `resolveComplypackPath()` -> `extractTarGz()`.

## Goals / Non-Goals

**Goals:**
- Add complypack content path resolution to the ampel provider's `Generate()` with precedence: `ComplypackContentPath` > `ampel_policy_dir` > default path
- Port the OPA provider's secure tar.gz extraction logic (`extractTarGz`, `resolveComplypackPath`) to the ampel provider
- Maintain full backward compatibility when `ComplypackContentPath` is empty
- Add comprehensive unit tests for extraction security and Generate integration

**Non-Goals:**
- Refactoring the ampel provider to use dependency-injected `ServerOptions` (separate concern)
- Adding a scan-config.json handoff artifact between Generate and Scan (the merged policy bundle path does not change)
- Extracting shared tar extraction code into a common internal package (can be done later once a third provider needs it)
- Changing the ampel policy file format or the `convert.LoadGranularPolicies()` function signature

## Decisions

### D1: Port extraction code rather than sharing it

Port the OPA provider's `resolveComplypackPath()` and `extractTarGz()` functions into a new `cmd/ampel-provider/server/unpack.go` file rather than extracting to an `internal/` shared package.

**Rationale:** The project convention is that providers are self-contained under `cmd/<name>-provider/` with no shared library code between them. `internal/complytime/` contains only test fixtures. Extracting to a shared package would be a structural change orthogonal to this feature. The two implementations can be unified later if a third consumer appears.

**Alternative considered:** Shared `internal/complypack/` package. Rejected because it changes the project's isolation model and is premature -- only two providers would use it.

### D2: Place extraction in a dedicated file

Create `cmd/ampel-provider/server/unpack.go` for `resolveComplypackPath()`, `extractTarGz()`, and `writeFileFromTar()` rather than inlining in `server.go`.

**Rationale:** The OPA provider places these functions in `server.go` alongside the main RPC logic, which already spans 700+ lines. Separating them into `unpack.go` keeps the ampel server file focused on RPC orchestration and makes the extraction logic independently testable in `unpack_test.go`.

### D3: Identical security constraints as OPA provider

Apply the same security hardening from the OPA provider's extraction:
- Path traversal rejection (no `../` or absolute paths in tar entries)
- Symlink and hard link rejection
- 100 MB per-file size limit
- Files created with mode `0600`
- Destination directory created with mode `0750`

**Rationale:** These are the established security requirements from the OPA implementation and match the issue's acceptance criteria. No reason to deviate.

### D4: No changes to convert package

The complypack content directory contains the same granular policy `.json` files that `convert.LoadGranularPolicies(sourceDir)` already consumes. Unlike the OPA provider (which needs a `complytime-mapping.json` indirection layer because Rego namespace IDs differ from requirement IDs), ampel policy IDs already match requirement IDs directly. The resolved complypack directory is simply passed as `sourceDir`.

**Rationale:** The existing `LoadGranularPolicies()` function signature accepts a directory path. The complypack content format matches what the function expects. No adaptation layer is needed.

## Risks / Trade-offs

- **[Code duplication]** The `extractTarGz` and `resolveComplypackPath` functions will be duplicated between OPA and ampel providers. -> Mitigation: Accept for now per project conventions; extract to shared package when a third provider needs it.
- **[Vendor dependency]** This change requires the vendored complyctl to include `ComplypackContentPath` on `GenerateRequest` (complytime-providers#38). -> Mitigation: The vendor update is a prerequisite tracked as a separate issue. Implementation should verify the field exists in the vendored types before proceeding.
- **[Idempotent extraction]** If a previous run left a corrupted `content/` directory, subsequent runs will reuse it without re-extracting. -> Mitigation: Same behavior as OPA provider. Users can delete the directory to force re-extraction. This is an acceptable trade-off for avoiding unnecessary I/O.
