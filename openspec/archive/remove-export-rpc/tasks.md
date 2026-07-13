## 1. Delete Export Packages

- [x] 1.1 Delete `cmd/openscap-provider/export/` directory (export.go, convert.go, export_test.go, convert_test.go)
- [x] 1.2 Delete `cmd/ampel-provider/export/` directory (export.go, convert.go, export_test.go, convert_test.go)

## 2. Remove Export from OpenSCAP Provider Server

- [x] 2.1 Remove `oscapexport` import from `cmd/openscap-provider/server/server.go`
- [x] 2.2 Remove `_ provider.Exporter = (*ProviderServer)(nil)` compile-time assertion from `cmd/openscap-provider/server/server.go`
- [x] 2.3 Remove `SupportsExport: true` from `Describe` response in `cmd/openscap-provider/server/server.go`
- [x] 2.4 Remove `Export()` method and `exportErrorMessage()` helper from `cmd/openscap-provider/server/server.go`
- [x] 2.5 Remove export-related tests from `cmd/openscap-provider/server/server_test.go`: `TestProviderServer_Describe_SupportsExport`, `setupExportServer`, `writeTestARF`, `TestExport_MissingARFFile`, `TestExport_NoRuleResults`, `TestExport_WithResults`, `TestExport_MalformedARF`, `TestExport_SkipsNotselectedResults`, `TestExportErrorMessage`. **WARNING: PRESERVE `TestRuleResultMessage` — it tests `xccdf.RuleResultMessage`, which is also called from the scan path (`server.go:226`). Do NOT bulk-delete the entire export tests section.**
- [x] 2.6 Update GoDoc comment in `cmd/openscap-provider/xccdf/arf.go:41` to remove "and export evidence" clause (stale reference to deleted capability)

## 3. Remove Export from AMPEL Provider Server

- [x] 3.1 Remove `ampelexport` import from `cmd/ampel-provider/server/server.go`
- [x] 3.2 Remove `_ provider.Exporter = (*ProviderServer)(nil)` compile-time assertion from `cmd/ampel-provider/server/server.go`
- [x] 3.3 Remove `SupportsExport: true` from `Describe` response in `cmd/ampel-provider/server/server.go`
- [x] 3.4 Remove `Export()` method and `exportErrorMessage()` helper from `cmd/ampel-provider/server/server.go`
- [x] 3.5 Remove export-related tests from `cmd/ampel-provider/server/server_test.go`: `TestDescribe_SupportsExport`, `writeExportResultFile`, `TestExport_NoResults`, `TestExport_WithResults`, `TestExport_MalformedResults`, `TestExport_EmptyFindingsInResult`, `TestExportErrorMessage`

## 4. Dependencies

- [x] 4.1 Remove `github.com/complytime/complybeacon/proofwatch` from `go.mod`
- [x] 4.2 Remove `github.com/gemaraproj/go-gemara` from `go.mod`
- [x] 4.3 Remove `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` from `go.mod`
- [x] 4.4 Remove `go.opentelemetry.io/otel/sdk/log` from `go.mod`
- [x] 4.5 Run `go mod tidy && go mod vendor` to prune transitive dependencies and update vendor directory

## 5. Documentation

- [x] 5.1 Update `AGENTS.md`: remove "optional Export RPC" and "proofwatch" references, remove `export/` entries from project structure tree
- [x] 5.2 Update `README.md`: remove OTLP export paragraph and Export RPC references
- [x] 5.3 Update `docs/provider-guide.md`: remove "Export Interface (Optional)" section and export references in provider table
- [x] 5.4 Add breaking change entry to `CHANGELOG.md` under `## Unreleased / ### Breaking Changes`. Entry should include: (1) what was removed — Export RPC implementation and `SupportsExport` field from OpenSCAP and AMPEL providers, (2) why — upstream complyctl removed the Exporter interface (complyctl PR #617, issue #606), (3) migration — no action required since complyctl no longer calls Export; remove any local tooling that checks `SupportsExport`, (4) dependencies removed — proofwatch, go-gemara, OTEL log SDK

## 6. OpenSpec Housekeeping

- [x] 6.1 Update `openspec/changes/add-export-rpc/.openspec.yaml` status from `implemented` to `superseded` (completed during spec preparation)

## 7. Verification

- [x] 7.1 Run `go build ./...` to confirm all providers compile without export references
- [x] 7.2 Run `go test ./...` to confirm all remaining tests pass
- [x] 7.3 Run `go vet ./...` to confirm no vet issues
- [x] 7.4 Run `make lint` to confirm no lint issues
- [x] 7.5 Grep for residual export references in Go source: `grep -rn "Export\|proofwatch\|CollectorConfig\|SupportsExport" --include="*.go" cmd/ internal/` and separately `grep -rn "Gemara\|gemara" --include="*.go" cmd/openscap-provider/ cmd/ampel-provider/`. NOTE: OPA provider uses `gemaraID` and "Gemara" in comments for requirement ID mapping — these are non-export references and expected.
- [x] 7.6 Grep for residual export references in documentation: `grep -rn "Export RPC\|SupportsExport\|proofwatch\|OTLP export\|evidence export\|provider\.Exporter" --include="*.md" . --exclude-dir=vendor --exclude-dir=openspec`
- [x] 7.7 Regenerate `.gaze/baseline.json` if gaze tooling is available
<!-- spec-review: passed -->
<!-- code-review: passed -->
