## ADDED Requirements

### Requirement: OpenSCAP provider declares export support
The OpenSCAP provider's `Describe` RPC response SHALL set `SupportsExport: true` so that complyctl includes it in the export phase when `--format otel` is used.

#### Scenario: Describe reports export capability
- **GIVEN** the OpenSCAP provider is running
- **WHEN** complyctl calls `Describe` on the OpenSCAP provider
- **THEN** the response includes `SupportsExport: true` alongside the existing health, version, and required variable fields

### Requirement: OpenSCAP provider implements Export RPC
The OpenSCAP provider `ProviderServer` SHALL implement the `provider.Exporter` interface by providing an `Export(ctx, *ExportRequest) (*ExportResponse, error)` method. The method SHALL read OpenSCAP scan results from the workspace ARF XML file, convert each assessment to a `GemaraEvidence` object, and emit them as OTLP log records via ProofWatch to the collector endpoint specified in the `ExportRequest.Collector` config.

#### Scenario: Successful export of OpenSCAP scan results
- **GIVEN** a successful scan has produced an ARF file at `.complytime/openscap/results/arf.xml`
- **WHEN** complyctl calls `Export` with a valid `CollectorConfig` (endpoint and auth token)
- **THEN** the OpenSCAP provider reads and parses the ARF file, converts each rule-result assessment to a `GemaraEvidence` with the correct Gemara attribute mappings, emits them via ProofWatch, and returns an `ExportResponse` with `Success: true` and `ExportedCount` equal to the number of evidence records emitted

#### Scenario: Export with no ARF file
- **GIVEN** no ARF file exists at the expected path
- **WHEN** complyctl calls `Export` with a valid `CollectorConfig`
- **THEN** the provider returns `ExportResponse` with `Success: false`, `ExportedCount: 0`, `FailedCount: 0`, and `ErrorMessage` indicating no scan results are available for export

#### Scenario: Export with collector connection failure
- **GIVEN** a successful scan has produced an ARF file
- **WHEN** complyctl calls `Export` with a `CollectorConfig` pointing to an unreachable endpoint
- **THEN** the provider returns `ExportResponse` with `Success: false` and `ErrorMessage` describing the connection failure

#### Scenario: Export with malformed ARF XML
- **GIVEN** an ARF file exists at the expected path but contains invalid or unparseable XML
- **WHEN** complyctl calls `Export` with a valid `CollectorConfig`
- **THEN** the provider returns `ExportResponse` with `Success: false` and `ErrorMessage` describing the parse failure

#### Scenario: Partial export failure
- **GIVEN** a successful scan has produced an ARF file with multiple rule-results
- **WHEN** some evidence records fail to emit via ProofWatch (e.g., collector becomes unavailable mid-export)
- **THEN** the provider returns `ExportResponse` with `Success: false`, `ExportedCount` reflecting successfully emitted records, `FailedCount` reflecting failed records, and `ErrorMessage` describing the failure count

### Requirement: OpenSCAP evidence carries correct Gemara attributes
Each `GemaraEvidence` record emitted by the OpenSCAP provider SHALL carry the following attributes mapped from the scan results:

- `policy.engine.name` SHALL be set to `"openscap"` via `Metadata.Author.Name`
- `compliance.control.id` SHALL be set to the requirement ID extracted from the OVAL check-content-ref name, via `AssessmentLog.Requirement.EntryId`
- `policy.rule.id` SHALL be set to the XCCDF rule ID ref, via `AssessmentLog.Plan.EntryId`
- `policy.evaluation.result` SHALL map `pass`/`fixed` to `gemara.Passed`, `fail` to `gemara.Failed`, and `error`/`unknown` to `gemara.Unknown`, via `AssessmentLog.Result`
- `compliance.assessment.id` SHALL be a deterministic composite identifier per evidence record (format: `openscap-<requirementID>-<ruleIDRef>`) via `Metadata.Id`, enabling idempotent re-export
- `policy.evaluation.message` SHALL include the rule title and any diagnostic messages from the rule-result, via `AssessmentLog.Message`

Note: The OTEL attribute names above are the wire-format attributes produced by ProofWatch internally. The provider sets the Gemara struct fields; ProofWatch handles the OTEL mapping. The Export path uses `gemara.Result` types (Passed/Failed/Unknown), which differ from the Scan path's `provider.Result` types (ResultPassed/ResultFailed/ResultError).

#### Scenario: Passed rule-result maps to correct attributes
- **GIVEN** an OpenSCAP scan has produced a rule-result with status `pass` for XCCDF rule `xccdf_org.ssgproject.content_rule_audit_rules_login_events` with OVAL-derived requirement ID `audit_rules_login_events`
- **WHEN** the rule-result is converted to `GemaraEvidence`
- **THEN** the evidence has `Metadata.Author.Name = "openscap"`, `AssessmentLog.Requirement.EntryId = "audit_rules_login_events"`, `AssessmentLog.Plan.EntryId = "xccdf_org.ssgproject.content_rule_audit_rules_login_events"`, and `AssessmentLog.Result = gemara.Passed`

#### Scenario: Failed rule-result maps to correct attributes
- **GIVEN** an OpenSCAP scan has produced a rule-result with status `fail` for a rule with title "Ensure audit rules for login events"
- **WHEN** the rule-result is converted to `GemaraEvidence`
- **THEN** the evidence has `AssessmentLog.Result = gemara.Failed` and `AssessmentLog.Message` includes the rule title

#### Scenario: Skippable results are excluded from export
- **GIVEN** an OpenSCAP scan has produced rule-results with status `notselected` or `notapplicable`
- **WHEN** the ARF file is processed for export
- **THEN** no `GemaraEvidence` records are emitted for those results, consistent with the existing scan behavior
