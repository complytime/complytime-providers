## 1. Delete Export Packages

- [ ] 1.1 Delete `cmd/openscap-provider/export/` directory (export.go, convert.go, export_test.go, convert_test.go)
- [ ] 1.2 Delete `cmd/ampel-provider/export/` directory (export.go, convert.go, export_test.go, convert_test.go)

## 2. Remove Export from OpenSCAP Provider Server

- [ ] 2.1 Remove `oscapexport` import from `cmd/openscap-provider/server/server.go`
- [ ] 2.2 Remove `_ provider.Exporter = (*ProviderServer)(nil)` compile-time assertion from `cmd/openscap-provider/server/server.go`
- [ ] 2.3 Remove `SupportsExport: true` from `Describe` response in `cmd/openscap-provider/server/server.go`
- [ ] 2.4 Remove `Export()` method and `exportErrorMessage()` helper from `cmd/openscap-provider/server/server.go`
- [ ] 2.5 Remove export-related tests from `cmd/openscap-provider/server/server_test.go`: `TestProviderServer_Describe_SupportsExport`, `setupExportServer`, `writeTestARF`, `TestExport_MissingARFFile`, `TestExport_NoRuleResults`, `TestExport_WithResults`, `TestExport_MalformedARF`, `TestExport_SkipsNotselectedResults`, `TestExportErrorMessage`

## 3. Remove Export from AMPEL Provider Server

- [ ] 3.1 Remove `ampelexport` import from `cmd/ampel-provider/server/server.go`
- [ ] 3.2 Remove `_ provider.Exporter = (*ProviderServer)(nil)` compile-time assertion from `cmd/ampel-provider/server/server.go`
- [ ] 3.3 Remove `SupportsExport: true` from `Describe` response in `cmd/ampel-provider/server/server.go`
- [ ] 3.4 Remove `Export()` method and `exportErrorMessage()` helper from `cmd/ampel-provider/server/server.go`
- [ ] 3.5 Remove export-related tests from `cmd/ampel-provider/server/server_test.go`: `TestDescribe_SupportsExport`, `writeExportResultFile`, `TestExport_NoResults`, `TestExport_WithResults`, `TestExport_MalformedResults`, `TestExport_EmptyFindingsInResult`, `TestExportErrorMessage`

## 4. Dependencies

- [ ] 4.1 Remove `github.com/complytime/complybeacon/proofwatch` from `go.mod`
- [ ] 4.2 Remove `github.com/gemaraproj/go-gemara` from `go.mod`
- [ ] 4.3 Remove `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` from `go.mod`
- [ ] 4.4 Remove `go.opentelemetry.io/otel/sdk/log` from `go.mod`
- [ ] 4.5 Run `go mod tidy && go mod vendor` to prune transitive dependencies and update vendor directory

## 5. Documentation

- [ ] 5.1 Update `AGENTS.md`: remove "optional Export RPC" and "proofwatch" references, remove `export/` entries from project structure tree
- [ ] 5.2 Update `README.md`: remove OTLP export paragraph and Export RPC references
- [ ] 5.3 Update `docs/provider-guide.md`: remove "Export Interface (Optional)" section and export references in provider table
- [ ] 5.4 Add breaking change entry to `CHANGELOG.md` under `## Unreleased / ### Breaking Changes`

## 6. OpenSpec Housekeeping

- [ ] 6.1 Update `openspec/changes/add-export-rpc/.openspec.yaml` status from `implemented` to `superseded`

## 7. Verification

- [ ] 7.1 Run `go build ./...` to confirm all providers compile without export references
- [ ] 7.2 Run `go test -race ./...` to confirm all remaining tests pass
- [ ] 7.3 Run `go vet ./...` to confirm no vet issues
- [ ] 7.4 Run `make lint` to confirm no lint issues
- [ ] 7.5 Grep for residual export references: `grep -rn "Export\|proofwatch\|Gemara\|gemara\|CollectorConfig\|SupportsExport" --include="*.go" cmd/ internal/`
- [ ] 7.6 Regenerate `.gaze/baseline.json` if gaze tooling is available
