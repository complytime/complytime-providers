## Context

The complyctl plugin SDK (vendored at `vendor/github.com/complytime/complyctl/pkg/provider/`) already defines the full Export RPC contract:

- `Exporter` interface with `Export(ctx, *ExportRequest) (*ExportResponse, error)`
- `ExportRequest` carrying `CollectorConfig` (endpoint + auth token)
- `ExportResponse` with success/count/error fields
- `grpcServer.Export()` does a runtime type assertion to check if a provider implements `Exporter`
- `DescribeResponse.SupportsExport` boolean for capability discovery
- `Manager.RouteExport()` for dispatching to capable plugins

Neither the AMPEL nor OpenSCAP provider implements the `Exporter` interface today. Both providers produce scan results (assessment logs) that need to be converted to `GemaraEvidence` objects and emitted via the ProofWatch library as OTLP log records to a Beacon collector.

ProofWatch (`github.com/complytime/complybeacon/proofwatch`) provides the `GemaraEvidence` type wrapping `gemara.Metadata` + `gemara.AssessmentLog`, and the `ProofWatch.Log(ctx, evidence)` method that emits OTEL log records through a configurable `LoggerProvider`.

## Goals / Non-Goals

### Goals

- Both providers satisfy the `provider.Exporter` interface
- Both providers declare `SupportsExport: true` in `Describe`
- Scan results are converted to `GemaraEvidence` using the correct Gemara attribute mappings
- OTEL log records are emitted directly from the plugin process to the collector via OTLP gRPC
- Export logic is unit-testable with a noop LoggerProvider
- Existing scan/generate behavior is completely unaffected

### Non-Goals

- No changes to the vendored complyctl SDK or proto definitions
- No custom OTLP transport or retry logic (the OTEL SDK handles this)
- No enrichment logic (the collector handles OCSF normalization and export)
- No OTLP HTTP fallback (gRPC only for initial implementation)
- No end-to-end integration tests against a real collector (unit tests with noop providers only)

## Decisions

### D1: Shared `export` package per provider vs. a common shared package

Each provider gets its own `export/` package (`cmd/ampel-provider/export/` and `cmd/openscap-provider/export/`). The conversion logic is provider-specific — AMPEL reads JSON attestation results from disk while OpenSCAP reads ARF XML results. The following code is duplicated across both providers and accepted at the current provider count (2). If a third provider is added, extract to `internal/export/`:

- `export/export.go` — `Emitter` struct, `NewEmitter`, `Shutdown` (~71 lines, byte-for-byte identical)
- `export/export_test.go` — Emitter unit tests (~100 lines, structurally identical)
- `server.go` `Export` method — orchestration pattern: read → convert → emit → count → respond (~55 lines, structurally identical with provider-specific read call)
- `server.go` `exportErrorMessage` helper — error message formatting (~5 lines, byte-for-byte identical)

Additionally, the OpenSCAP provider has two `mapResultStatus` functions — one in `server.go` (returns `provider.Result` for the Scan path) and one in `export/convert.go` (returns `gemara.Result` for the Export path). These map the same XCCDF result strings but to different target types. This intra-provider duplication is accepted because the type boundary makes a shared function impractical without generics or interface indirection.

**Alternative considered**: A single `internal/export/` shared package. Rejected because the result-to-evidence conversion is different for each provider, and a shared package would need provider-specific interfaces adding unnecessary indirection.

### D2: OTEL SDK setup lives in the export package

Each provider's `export` package creates the OTLP gRPC log exporter, `sdklog.LoggerProvider`, and `ProofWatch` instance from the `CollectorConfig` received in the `ExportRequest`. The setup and teardown happen within the scope of a single `Export` RPC call. The `LoggerProvider` is shut down (flushing buffered logs) before the RPC returns. The shutdown timeout is bounded at 10 seconds.

The OTLP gRPC exporter uses TLS by default (no `WithInsecure()` option). The endpoint format is `host:port` as expected by the OTLP gRPC library. Endpoint validation is delegated to the OTLP gRPC library, which will fail gracefully on invalid endpoints.

**Alternative considered**: Initializing the OTEL stack once at provider startup and reusing across calls. Rejected because Export is called at most once per scan invocation, and per-call setup avoids state management complexity. The collector config (endpoint, token) comes from the `ExportRequest`, which is only available at call time.

### D3: Scan results are read from workspace disk, not in-memory state

Both providers write scan results to the workspace directory during `Scan` (AMPEL writes per-repo JSON, OpenSCAP writes ARF XML). The `Export` method reads these same files to build `GemaraEvidence` objects. This avoids adding mutable state to the `ProviderServer` struct and aligns with the existing stateless-server pattern.

Note: The two providers handle "no results" differently. AMPEL treats an empty or missing results directory as "zero results to export" (`Success: true, ExportedCount: 0`) because AMPEL scans multiple repos and an empty directory is a valid state. OpenSCAP treats a missing ARF file as an error (`Success: false`) because the scan is expected to produce exactly one ARF file. This asymmetry reflects the structural difference between the two scan result formats.

**Alternative considered**: Storing scan results in memory on the `ProviderServer` between Scan and Export calls. Rejected because it introduces mutable state, complicates testing, and the files are already written as part of the scan flow.

### D4: Bearer token auth via OTLP exporter headers

The `CollectorConfig.AuthToken` (a resolved bearer token from complyctl's OIDC exchange) is passed as an `Authorization: Bearer <token>` header via `otlploggrpc.WithHeaders()`. This is the standard OTLP gRPC auth mechanism.

### D5: Result-to-Gemara mapping

Provider scan results map to Gemara struct fields as follows. The OTEL attribute column shows the wire-format output produced by ProofWatch internally — providers set the Gemara struct fields, not the OTEL attributes directly.

| Provider Source | Gemara Field | OTEL Attribute (via ProofWatch) |
|---|---|---|
| Provider name (`"ampel"` / `"openscap"`) | `Metadata.Author.Name` | `policy.engine.name` |
| AMPEL: `TenetID` (stripped `check-` prefix); OpenSCAP: OVAL check-content-ref name | `AssessmentLog.Requirement.EntryId` | `compliance.control.id` |
| AMPEL: same requirement ID; OpenSCAP: XCCDF rule ID ref | `AssessmentLog.Plan.EntryId` | `policy.rule.id` |
| AMPEL: `"pass"`/`"fail"`/default; OpenSCAP: `pass`/`fixed`/`fail`/`error`/`unknown` | `AssessmentLog.Result` (`gemara.Passed`/`Failed`/`Unknown`) | `policy.evaluation.result` |
| Step messages (AMPEL) / rule title + diagnostics (OpenSCAP) | `AssessmentLog.Message` | `policy.evaluation.message` |
| Deterministic composite key (AMPEL: `ampel-<reqID>-<repo>-<branch>`; OpenSCAP: `openscap-<reqID>-<ruleIDRef>`) | `Metadata.Id` | `compliance.assessment.id` |

## Risks / Trade-offs

- **[Risk] OTEL SDK version compatibility**: ProofWatch depends on experimental OTEL log SDK packages (`go.opentelemetry.io/otel/sdk/log`, `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc`). These are pre-1.0 (`v0.19.0`) with no API stability guarantee. Breaking changes between minor versions are expected. → **Mitigation**: Pin to `v0.19.0` for the log-related experimental packages and align version updates with ProofWatch's dependency versions. When ProofWatch updates its OTEL dependency, update both providers in lockstep. If a breaking API change is incompatible with ProofWatch, defer the update until ProofWatch adapts.

- **[Risk] Per-call OTEL setup overhead**: Creating and shutting down the OTLP exporter and LoggerProvider for each Export call adds latency (connection setup, TLS handshake, flush timeout). → **Mitigation**: Acceptable for the expected use case (one Export call per scan run). The batch processor flush timeout is bounded at 10 seconds.

- **[Trade-off] Disk-based result reading**: Reading results from disk couples Export to the file layout written by Scan. If the file layout changes, Export must change too. → **Mitigation**: Both are in the same provider binary and tested together. The coupling is intentional and local.

- **[Trade-off] Duplicated Emitter code**: The `export.go` OTEL setup code is byte-for-byte identical across both providers (~67 lines). This is accepted at the current provider count (2) to avoid premature abstraction. If a third provider is added, extract to `internal/export/`.
