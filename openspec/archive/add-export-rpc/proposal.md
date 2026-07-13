## Why

Both the AMPEL and OpenSCAP provider plugins currently lack Export RPC support. The complyctl SDK already defines the `Exporter` interface and the gRPC plumbing (`Export` RPC, `ExportRequest`/`ExportResponse`, `CollectorConfig`), but neither plugin implements it. When `complyctl scan --format otel` is used, these plugins are skipped. Implementing Export in both plugins enables direct evidence shipping to a Beacon collector via ProofWatch, completing the end-to-end compliance evidence pipeline without manual transfer.

## What Changes

- Add `Export` method to the AMPEL provider (`cmd/ampel-provider/server/server.go`) that satisfies the `provider.Exporter` interface
- Add `Export` method to the OpenSCAP provider (`cmd/openscap-provider/server/server.go`) that satisfies the `provider.Exporter` interface
- Set `SupportsExport: true` in both providers' `Describe` responses
- Add ProofWatch dependency (`github.com/complytime/complybeacon/proofwatch`) and Gemara dependency (`github.com/gemaraproj/go-gemara`) to `go.mod`
- Add an `export` package to each provider for converting scan results to `GemaraEvidence` and emitting via ProofWatch
- Add unit tests for the export logic in both providers

## Capabilities

### New Capabilities

- `ampel-export`: AMPEL provider implements the Export RPC, converting AMPEL scan results to GemaraEvidence and emitting them as OTLP log records via ProofWatch
- `openscap-export`: OpenSCAP provider implements the Export RPC, converting OpenSCAP scan results to GemaraEvidence and emitting them as OTLP log records via ProofWatch

### Modified Capabilities

- `ampel-describe`: `DescribeResponse` now includes `SupportsExport: true`
- `openscap-describe`: `DescribeResponse` now includes `SupportsExport: true`

### Removed Capabilities

None.

## Impact

- **Code**: `cmd/ampel-provider/server/server.go`, `cmd/openscap-provider/server/server.go` gain Export methods; new `export/` packages in each provider
- **Dependencies**: `github.com/complytime/complybeacon/proofwatch`, `github.com/gemaraproj/go-gemara`, and transitive OTEL SDK dependencies added to `go.mod`
- **APIs**: No proto changes needed; the Export RPC is already defined in the vendored complyctl SDK. Both providers now declare `supports_export: true`
- **Testing**: New unit tests for export conversion and ProofWatch integration; existing tests remain unchanged
- **Documentation**: `docs/provider-guide.md` must be updated to document the `Exporter` interface, the Export RPC, and the ProofWatch dependency pattern

## Constitution Alignment

Assessed against the Unbound Force org constitution.

### I. Autonomous Collaboration

**Assessment**: PASS

Each provider's export package is self-contained with its own conversion logic and OTEL setup. The Export RPC produces self-describing OTLP log records with structured Gemara attributes — artifact-based communication that any OTLP-compatible collector can consume without provider-specific knowledge.

### II. Composability First

**Assessment**: PASS

Export is an optional capability (providers opt in via `SupportsExport: true` and the `Exporter` interface). Providers that do not implement Export continue to work unchanged. The per-provider `export/` package pattern keeps conversion logic isolated — each provider can evolve its mapping independently. ProofWatch is the only new mandatory dependency for providers that implement Export, and it serves as a thin adapter to the OTEL SDK.

### III. Observable Quality

**Assessment**: PASS

This change directly improves observable quality by enabling machine-parseable OTLP evidence emission. Each evidence record carries structured provenance metadata (engine name, requirement ID, assessment result, deterministic assessment ID) as OTEL log record attributes. The deterministic assessment ID enables idempotent re-export and deduplication at the collector.

### IV. Testability

**Assessment**: PASS

The OTEL SDK's noop LoggerProvider enables unit testing of the full export pipeline without a running collector. Each provider's conversion logic is tested in isolation via `convert_test.go`. The `Export` method on `ProviderServer` is tested at the server level. Compile-time interface assertions (`var _ provider.Exporter`) catch interface drift at build time.
