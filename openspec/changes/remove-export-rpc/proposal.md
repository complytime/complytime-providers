## Why

The upstream `complyctl` removed the entire Export RPC infrastructure in PR [#617](https://github.com/complytime/complyctl/pull/617) (fixes [complyctl#606](https://github.com/complytime/complyctl/issues/606)). The `Exporter` interface, `ExportRequest`/`ExportResponse` types, `CollectorConfig`, and `SupportsExport` field no longer exist in the `complyctl` provider SDK. Both the OpenSCAP and AMPEL providers in this repository implement that removed interface and reference those deleted types. The providers will fail to compile when the `complyctl` dependency is bumped past `v1.0.0-beta.0`. This removal must land before any `complyctl` dependency bump.

## What Changes

- **BREAKING**: Delete `cmd/openscap-provider/export/` package (4 files: `export.go`, `convert.go`, `export_test.go`, `convert_test.go`)
- **BREAKING**: Delete `cmd/ampel-provider/export/` package (4 files: `export.go`, `convert.go`, `export_test.go`, `convert_test.go`)
- Remove `Export()` method and `exportErrorMessage()` helper from `cmd/openscap-provider/server/server.go`
- Remove `Export()` method and `exportErrorMessage()` helper from `cmd/ampel-provider/server/server.go`
- Remove `_ provider.Exporter` compile-time interface assertions from both server files
- Remove `SupportsExport: true` from both providers' `Describe` responses
- Remove export-related tests from both providers' `server_test.go` files
- Remove export-only dependencies from `go.mod`: `proofwatch`, `go-gemara`, `otlploggrpc`, `otel/sdk/log`
- Run `go mod tidy && go mod vendor` to prune transitive dependencies
- Update `README.md`, `AGENTS.md`, `docs/provider-guide.md` to remove export references
- Add breaking change entry to `CHANGELOG.md`
- Mark `openspec/changes/add-export-rpc` as `SUPERSEDED`
- OPA provider: no changes needed (never implemented Export)

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The Describe RPC contract for both providers narrows (drops `SupportsExport`) but this is a removal, not a behavioral change to remaining capabilities.

### Removed Capabilities

- `openscap-export`: OpenSCAP provider's Export RPC implementation, ARF-to-GemaraEvidence conversion, and OTLP emission via ProofWatch
- `ampel-export`: AMPEL provider's Export RPC implementation, result-to-GemaraEvidence conversion, and OTLP emission via ProofWatch

These capabilities were added in `openspec/changes/add-export-rpc` (status: implemented). That change is now superseded by the upstream removal.

## Impact

- **Code**: 8 files deleted entirely, 4 files modified (server.go + server_test.go for openscap and ampel providers)
- **Dependencies**: 4 direct dependencies removed (`proofwatch`, `go-gemara`, `otlploggrpc`, `otel/sdk/log`); transitive dependencies pruned by `go mod tidy` (`go-ocsf`, OTEL core packages)
- **APIs**: Both providers stop declaring `SupportsExport: true` and stop implementing the `Exporter` interface. This is transparent to users since complyctl no longer calls Export.
- **Testing**: ~13 export-related tests removed across both providers. All remaining tests must continue to pass.
- **Documentation**: Export sections removed from provider guide, README, and AGENTS.md. CHANGELOG updated with breaking change entry.
- **Upstream coordination**: Relates to [complyctl#650](https://github.com/complytime/complyctl/issues/650) (post-removal cleanup tracking in complyctl)
